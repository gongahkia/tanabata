import json
from pathlib import Path

from bot.models import normalize_food_payload


REPO_ROOT = Path(__file__).resolve().parents[1]
OUTPUT_DIR = REPO_ROOT / "scrapers" / "output"


def test_normalize_food_payload_dedupes_flat_payloads():
    payload = [
        {
            "name": "Cafe A",
            "location": "Level 1",
            "description": "Fresh bowls",
            "category": "Cafe",
            "url": "https://example.com/a",
        },
        {
            "name": "Cafe A",
            "location": "Level 1",
            "description": "Fresh bowls",
            "category": "Cafe",
            "url": "https://example.com/a",
        },
    ]

    items, errors = normalize_food_payload(payload)

    assert errors == []
    assert len(items) == 1
    assert items[0].name == "Cafe A"


def test_normalize_food_payload_handles_nested_nus_fixture():
    payload = json.loads((OUTPUT_DIR / "nus_dining_details.json").read_text())

    items, errors = normalize_food_payload(payload)

    assert len(items) > 10
    assert errors == []
    assert all(item.name for item in items)
    assert all(item.category for item in items)


def test_normalize_food_payload_handles_foodadvisor_lists():
    payload = json.loads((OUTPUT_DIR / "sim_dining_data.json").read_text())

    items, errors = normalize_food_payload(payload[:3])

    assert errors == []
    assert len(items) == 3
    assert "S$5 - S$15" in items[0].description
    assert items[0].category

