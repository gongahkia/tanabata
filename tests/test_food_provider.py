import asyncio
import json
from datetime import datetime, timezone
from pathlib import Path

from bot.area_registry import load_area_registry
from bot.food_provider import FoodProvider
from bot.models import AreaDefinition, FoodPlace, ScraperResult
from bot.config import Settings


REPO_ROOT = Path(__file__).resolve().parents[1]


def make_settings(tmp_path: Path) -> Settings:
    return Settings(bot_token="123:test-token", bot_state_path=tmp_path / ".state" / "bot_state.pkl")


def make_area(seed_file: str | None = "smu_dining_details.json") -> AreaDefinition:
    return AreaDefinition(
        id="smu_campus",
        label="SMU Campus",
        category="campus",
        latitude=1.2966,
        longitude=103.8496,
        scraper_module="smu_shakespeare",
        source_urls=("https://example.com",),
        seed_file=seed_file,
    )


def test_provider_falls_back_to_seed_data(tmp_path):
    settings = make_settings(tmp_path)
    provider = FoodProvider(settings)
    area = make_area()

    async def fake_live(_area):
        return ScraperResult(items=[], source_status="live", errors=["live failed"])

    provider._run_live_scraper = fake_live  # type: ignore[method-assign]

    result = asyncio.run(provider.get_food_places(area))

    assert result.source_status == "seed"
    assert result.items
    assert "live failed" in result.errors[0]


def test_provider_prefers_fresh_cache_over_seed(tmp_path):
    settings = make_settings(tmp_path)
    settings.cache_dir.mkdir(parents=True, exist_ok=True)
    provider = FoodProvider(settings)
    area = make_area()
    cache_path = settings.cache_dir / f"{area.id}.json"
    payload = {
        "fetched_at": datetime.now(timezone.utc).isoformat(),
        "items": [
            {
                "name": "Cached Cafe",
                "location": "Cached Mall",
                "description": "From cache",
                "category": "Cafe",
                "url": "https://cached.example.com",
            }
        ],
    }
    cache_path.write_text(json.dumps(payload))

    async def fake_live(_area):
        return ScraperResult(items=[], source_status="live", errors=["live failed"])

    provider._run_live_scraper = fake_live  # type: ignore[method-assign]

    result = asyncio.run(provider.get_food_places(area))

    assert result.source_status == "cached"
    assert result.items[0].name == "Cached Cafe"


def test_provider_writes_cache_on_live_success(tmp_path):
    settings = make_settings(tmp_path)
    provider = FoodProvider(settings)
    area = make_area()

    async def fake_live(_area):
        return ScraperResult(
            items=[
                FoodPlace(
                    name="Live Cafe",
                    location="Live Mall",
                    description="From live scraper",
                    category="Cafe",
                    url="https://live.example.com",
                )
            ],
            source_status="live",
        )

    provider._run_live_scraper = fake_live  # type: ignore[method-assign]

    result = asyncio.run(provider.get_food_places(area))

    assert result.source_status == "live"
    assert (settings.cache_dir / f"{area.id}.json").exists()


def test_provider_reads_combined_seed_payloads_for_shared_seed_files(tmp_path):
    settings = make_settings(tmp_path)
    provider = FoodProvider(settings)
    registry = load_area_registry()

    async def fake_live(_area):
        return ScraperResult(items=[], source_status="live", errors=["live failed"])

    provider._run_live_scraper = fake_live  # type: ignore[method-assign]

    for area_id in ("causeway_point", "plaza_singapura"):
        result = asyncio.run(provider.get_food_places(registry[area_id]))

        assert result.source_status == "seed"
        assert result.items
        assert result.items[0].name
