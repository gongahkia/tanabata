from __future__ import annotations

import os
from dataclasses import dataclass
from pathlib import Path

from dotenv import load_dotenv

try:
    from .area_registry import validate_area_registry
except ImportError:  # pragma: no cover - script execution fallback
    from area_registry import validate_area_registry


DEFAULT_SCRAPER_TIMEOUT_SECONDS = 20
DEFAULT_SCRAPER_CACHE_TTL_MINUTES = 720
DEFAULT_BOT_STATE_PATH = "./.state/bot_state.pkl"


@dataclass(frozen=True)
class Settings:
    bot_token: str
    scraper_timeout_seconds: int = DEFAULT_SCRAPER_TIMEOUT_SECONDS
    scraper_cache_ttl_minutes: int = DEFAULT_SCRAPER_CACHE_TTL_MINUTES
    bot_state_path: Path = Path(DEFAULT_BOT_STATE_PATH)

    @property
    def bot_dir(self) -> Path:
        return Path(__file__).resolve().parent

    @property
    def repo_root(self) -> Path:
        return self.bot_dir.parent

    @property
    def state_dir(self) -> Path:
        return self.bot_state_path.parent

    @property
    def cache_dir(self) -> Path:
        return self.state_dir / "cache"

    @property
    def seed_dir(self) -> Path:
        return self.repo_root / "scrapers" / "output"


def _read_positive_int(name: str, default: int) -> int:
    raw_value = os.getenv(name, str(default))
    try:
        parsed = int(raw_value)
    except ValueError as exc:
        raise RuntimeError(f"{name} must be an integer, got {raw_value!r}.") from exc
    if parsed <= 0:
        raise RuntimeError(f"{name} must be positive, got {parsed}.")
    return parsed


def load_settings() -> Settings:
    load_dotenv()
    bot_token = os.getenv("BOT_TOKEN")
    if not bot_token:
        raise RuntimeError("BOT_TOKEN is required.")

    return Settings(
        bot_token=bot_token,
        scraper_timeout_seconds=_read_positive_int(
            "SCRAPER_TIMEOUT_SECONDS", DEFAULT_SCRAPER_TIMEOUT_SECONDS
        ),
        scraper_cache_ttl_minutes=_read_positive_int(
            "SCRAPER_CACHE_TTL_MINUTES", DEFAULT_SCRAPER_CACHE_TTL_MINUTES
        ),
        bot_state_path=Path(os.getenv("BOT_STATE_PATH", DEFAULT_BOT_STATE_PATH)),
    )


async def validate_playwright_browser() -> None:
    from playwright.async_api import async_playwright

    async with async_playwright() as playwright:
        executable_path = Path(playwright.chromium.executable_path)
        if not executable_path.exists():
            raise RuntimeError(
                "Playwright Chromium browser is unavailable. "
                "Run `python -m playwright install chromium`."
            )


async def validate_runtime(settings: Settings, registry: dict[str, object]) -> None:
    settings.state_dir.mkdir(parents=True, exist_ok=True)
    settings.cache_dir.mkdir(parents=True, exist_ok=True)
    validate_area_registry(registry, seed_dir=settings.seed_dir)
    await validate_playwright_browser()
