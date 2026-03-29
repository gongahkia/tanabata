from pathlib import Path

import pytest

from bot.area_registry import load_area_registry, validate_area_registry
from bot.models import AreaDefinition


REPO_ROOT = Path(__file__).resolve().parents[1]


def test_area_registry_contains_supported_areas_only():
    registry = load_area_registry()

    assert len(registry) == 57
    assert "waterway_point" in registry
    assert registry["waterway_point"].seed_file == "frasers_store_links.json"
    assert "the_clementi_mall" in registry
    assert "ntu_campus" in registry
    assert registry["ntu_campus"].scraper_module == "seed_only_scraper"
    assert all(area.seed_file for area in registry.values())


def test_legacy_metadata_files_are_removed():
    assert not (REPO_ROOT / "bot" / "bot_details.json").exists()
    assert not (REPO_ROOT / "bot" / "locations.json").exists()


def test_area_registry_validation_rejects_missing_modules():
    registry = {
        "broken_area": AreaDefinition(
            id="broken_area",
            label="Broken Area",
            category="mall",
            latitude=1.3,
            longitude=103.8,
            scraper_module="missing_scraper_module",
            source_urls=("https://example.com",),
            seed_file=None,
        )
    }

    with pytest.raises(RuntimeError, match="missing_scraper_module"):
        validate_area_registry(registry)


def test_area_registry_validation_accepts_real_registry():
    registry = load_area_registry()

    validate_area_registry(registry, seed_dir=REPO_ROOT / "scrapers" / "output")
