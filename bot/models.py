from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime
from typing import Any, Iterable, Mapping, Sequence


def _normalize_text(value: Any, separator: str = " | ") -> str:
    if value is None:
        return ""
    if isinstance(value, str):
        return " ".join(value.split())
    if isinstance(value, Sequence) and not isinstance(value, (str, bytes)):
        parts = [_normalize_text(item, separator=separator) for item in value]
        return separator.join(part for part in parts if part)
    return " ".join(str(value).split())


def _looks_like_food_mapping(payload: Mapping[str, Any]) -> bool:
    return "name" in payload and any(
        key in payload
        for key in ("location", "description", "descriptions", "category", "categories", "url")
    )


@dataclass(frozen=True)
class FoodPlace:
    name: str
    location: str
    description: str
    category: str
    url: str

    @classmethod
    def from_raw(cls, payload: Mapping[str, Any]) -> "FoodPlace":
        name = _normalize_text(payload.get("name"))
        if not name:
            raise ValueError("Food payload is missing a name.")

        category_value = payload.get("category")
        if category_value is None:
            category_value = payload.get("categories")

        description_value = payload.get("description")
        if description_value is None:
            description_value = payload.get("descriptions")

        return cls(
            name=name,
            location=_normalize_text(payload.get("location")),
            description=_normalize_text(description_value),
            category=_normalize_text(category_value, separator=", "),
            url=_normalize_text(payload.get("url")),
        )

    def to_dict(self) -> dict[str, str]:
        return {
            "name": self.name,
            "location": self.location,
            "description": self.description,
            "category": self.category,
            "url": self.url,
        }


@dataclass(frozen=True)
class AreaDefinition:
    id: str
    label: str
    category: str
    latitude: float
    longitude: float
    scraper_module: str
    source_urls: tuple[str, ...]
    seed_file: str | None = None


@dataclass(frozen=True)
class AreaCandidate:
    area: AreaDefinition
    distance_km: float
    travel_minutes: float
    walkable: bool


@dataclass
class ScraperResult:
    items: list[FoodPlace]
    source_status: str
    errors: list[str] = field(default_factory=list)
    fetched_at: datetime | None = None

    def has_items(self) -> bool:
        return bool(self.items)


@dataclass(frozen=True)
class UserPrefs:
    pace_kmh: float | None = None
    walk_time_minutes: int | None = None
    last_area_id: str | None = None

    @classmethod
    def from_dict(cls, payload: Mapping[str, Any] | None) -> "UserPrefs":
        payload = payload or {}
        return cls(
            pace_kmh=payload.get("pace_kmh"),
            walk_time_minutes=payload.get("walk_time_minutes"),
            last_area_id=payload.get("last_area_id"),
        )

    def to_dict(self) -> dict[str, Any]:
        return {
            "pace_kmh": self.pace_kmh,
            "walk_time_minutes": self.walk_time_minutes,
            "last_area_id": self.last_area_id,
        }


def dedupe_food_places(items: Iterable[FoodPlace]) -> list[FoodPlace]:
    unique: list[FoodPlace] = []
    seen: set[tuple[str, str, str]] = set()
    for item in items:
        key = (item.name.casefold(), item.location.casefold(), item.url.casefold())
        if key in seen:
            continue
        seen.add(key)
        unique.append(item)
    return unique


def normalize_food_payload(payload: Any) -> tuple[list[FoodPlace], list[str]]:
    items: list[FoodPlace] = []
    errors: list[str] = []

    def visit(node: Any) -> None:
        if node is None:
            return
        if isinstance(node, Mapping):
            if _looks_like_food_mapping(node):
                try:
                    items.append(FoodPlace.from_raw(node))
                except ValueError as exc:
                    errors.append(str(exc))
                return
            for value in node.values():
                visit(value)
            return
        if isinstance(node, Sequence) and not isinstance(node, (str, bytes)):
            if node and all(isinstance(value, str) for value in node):
                errors.extend(value.strip() for value in node if value.strip())
                return
            for value in node:
                visit(value)
            return
        if isinstance(node, str):
            stripped = node.strip()
            if stripped:
                errors.append(stripped)

    visit(payload)
    return dedupe_food_places(items), errors

