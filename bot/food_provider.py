from __future__ import annotations

import asyncio
import importlib.util
import json
import logging
from datetime import datetime, timedelta, timezone
from pathlib import Path
from types import ModuleType

try:
    from .config import Settings
    from .models import AreaDefinition, FoodPlace, ScraperResult, normalize_food_payload
except ImportError:  # pragma: no cover - script execution fallback
    from config import Settings
    from models import AreaDefinition, FoodPlace, ScraperResult, normalize_food_payload


logger = logging.getLogger(__name__)


def _utcnow() -> datetime:
    return datetime.now(timezone.utc)


class FoodProvider:
    def __init__(self, settings: Settings):
        self.settings = settings
        self.scraper_dir = settings.bot_dir / "bot_scraper"
        self.seed_dir = settings.seed_dir
        self.cache_dir = settings.cache_dir
        self._module_cache: dict[str, ModuleType] = {}

    async def get_food_places(self, area: AreaDefinition) -> ScraperResult:
        live_result = await self._run_live_scraper(area)
        if live_result.has_items():
            self._write_cache(area, live_result.items)
            return live_result

        cached_result = self._load_cache(area)
        if cached_result.has_items():
            cached_result.errors = live_result.errors + cached_result.errors
            return cached_result

        seed_result = self._load_seed(area)
        if seed_result.has_items():
            seed_result.errors = live_result.errors + seed_result.errors
            return seed_result

        return ScraperResult(
            items=[],
            source_status="unavailable",
            errors=live_result.errors
            + cached_result.errors
            + seed_result.errors
            + [f"No food data is available for {area.label} right now."],
        )

    async def _run_live_scraper(self, area: AreaDefinition) -> ScraperResult:
        if area.scraper_module == "seed_only_scraper":
            return ScraperResult(
                items=[],
                source_status="live",
                errors=[
                    f"Live scraping is not configured for {area.label}; using local fallback data."
                ],
            )

        module_path = self.scraper_dir / f"{area.scraper_module}.py"
        if not module_path.exists():
            return ScraperResult(
                items=[],
                source_status="live",
                errors=[f"Scraper module {area.scraper_module!r} was not found."],
            )

        try:
            module = self._load_scraper_module(area.scraper_module, module_path)
            payload = await asyncio.wait_for(
                module.run_scraper(
                    area.source_urls[0]
                    if len(area.source_urls) == 1
                    else list(area.source_urls)
                ),
                timeout=self.settings.scraper_timeout_seconds,
            )
            items, errors = normalize_food_payload(payload)
            if not items:
                errors = errors or [f"Live scraper returned no eateries for {area.label}."]
            return ScraperResult(
                items=items,
                source_status="live",
                errors=errors,
                fetched_at=_utcnow(),
            )
        except asyncio.TimeoutError:
            logger.warning("Timed out while scraping %s", area.label)
            return ScraperResult(
                items=[],
                source_status="live",
                errors=[
                    f"Live scraper timed out after {self.settings.scraper_timeout_seconds} seconds."
                ],
            )
        except Exception as exc:  # pragma: no cover - defensive path
            logger.exception("Unexpected scraper failure for %s", area.label)
            return ScraperResult(
                items=[],
                source_status="live",
                errors=[f"Live scraper failed for {area.label}: {exc}"],
            )

    def _load_cache(self, area: AreaDefinition) -> ScraperResult:
        cache_path = self.cache_dir / f"{area.id}.json"
        if not cache_path.exists():
            return ScraperResult(items=[], source_status="cached")

        try:
            payload = json.loads(cache_path.read_text())
            fetched_at = datetime.fromisoformat(payload["fetched_at"])
            if fetched_at.tzinfo is None:
                fetched_at = fetched_at.replace(tzinfo=timezone.utc)
            if _utcnow() - fetched_at > timedelta(
                minutes=self.settings.scraper_cache_ttl_minutes
            ):
                return ScraperResult(
                    items=[],
                    source_status="cached",
                    errors=[f"Cached data for {area.label} is older than the TTL."],
                )

            items = [FoodPlace.from_raw(item) for item in payload.get("items", [])]
            return ScraperResult(
                items=items,
                source_status="cached",
                fetched_at=fetched_at,
            )
        except Exception as exc:
            logger.warning("Failed to read cache for %s: %s", area.label, exc)
            return ScraperResult(
                items=[],
                source_status="cached",
                errors=[f"Cached data for {area.label} could not be read."],
            )

    def _load_seed(self, area: AreaDefinition) -> ScraperResult:
        if not area.seed_file:
            return ScraperResult(items=[], source_status="seed")

        seed_path = self.seed_dir / area.seed_file
        if not seed_path.exists():
            return ScraperResult(
                items=[],
                source_status="seed",
                errors=[f"Seed data file {area.seed_file!r} is missing."],
            )

        try:
            payload = json.loads(seed_path.read_text())
            if isinstance(payload, dict):
                derived_key = area.id.replace("_", "")
                if area.id in payload:
                    payload = payload[area.id]
                elif derived_key in payload:
                    payload = payload[derived_key]
            items, errors = normalize_food_payload(payload)
            if not items:
                errors = errors or [f"Seed data for {area.label} is empty."]
            return ScraperResult(
                items=items,
                source_status="seed",
                errors=errors,
            )
        except Exception as exc:
            logger.warning("Failed to read seed data for %s: %s", area.label, exc)
            return ScraperResult(
                items=[],
                source_status="seed",
                errors=[f"Seed data for {area.label} could not be read."],
            )

    def _write_cache(self, area: AreaDefinition, items: list[FoodPlace]) -> None:
        cache_path = self.cache_dir / f"{area.id}.json"
        cache_path.parent.mkdir(parents=True, exist_ok=True)
        payload = {
            "fetched_at": _utcnow().isoformat(),
            "items": [item.to_dict() for item in items],
        }
        cache_path.write_text(json.dumps(payload, indent=2))

    def _load_scraper_module(self, module_name: str, module_path: Path) -> ModuleType:
        if module_name in self._module_cache:
            return self._module_cache[module_name]

        spec = importlib.util.spec_from_file_location(
            f"takko_scrapers.{module_name}", module_path
        )
        if spec is None or spec.loader is None:
            raise RuntimeError(f"Unable to load scraper module {module_name!r}.")
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        self._module_cache[module_name] = module
        return module
