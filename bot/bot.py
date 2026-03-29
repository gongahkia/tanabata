from __future__ import annotations

import asyncio
import logging
import random
from typing import Any

from telegram import (
    InlineKeyboardButton,
    InlineKeyboardMarkup,
    KeyboardButton,
    ReplyKeyboardMarkup,
    ReplyKeyboardRemove,
    Update,
)
from telegram.ext import (
    Application,
    ApplicationBuilder,
    CallbackQueryHandler,
    CommandHandler,
    ContextTypes,
    ConversationHandler,
    MessageHandler,
    PicklePersistence,
    filters,
)

try:
    from .area_registry import areas_by_category, load_area_registry
    from .config import Settings, load_settings, validate_runtime
    from .food_provider import FoodProvider
    from .geolocation import haversine, locations_near
    from .models import AreaCandidate, FoodPlace, UserPrefs
except ImportError:  # pragma: no cover - script execution fallback
    from area_registry import areas_by_category, load_area_registry
    from config import Settings, load_settings, validate_runtime
    from food_provider import FoodProvider
    from geolocation import haversine, locations_near
    from models import AreaCandidate, FoodPlace, UserPrefs


logging.basicConfig(
    format="%(asctime)s %(levelname)s [%(name)s] %(message)s",
    level=logging.INFO,
)
logger = logging.getLogger(__name__)


LOCATION_OR_AREA, PACE, WALK_TIME, INTENT, RESULTS = range(5)

AREA_PAGE_SIZE = 8
PACE_PRESETS = (
    ("Slow", 4.5),
    ("Average", 5.5),
    ("Brisk", 6.5),
)
WALK_TIME_PRESETS = (5, 10, 15, 20, 30)
FAST_LUNCH_MINUTES = 15
DEFAULT_FAST_PACE_KMH = 5.5


def _get_registry(context: ContextTypes.DEFAULT_TYPE) -> dict[str, Any]:
    return context.application.bot_data["registry"]


def _get_provider(context: ContextTypes.DEFAULT_TYPE) -> FoodProvider:
    return context.application.bot_data["food_provider"]


def _profile_key() -> str:
    return "profile"


def get_user_prefs(context: ContextTypes.DEFAULT_TYPE) -> UserPrefs:
    return UserPrefs.from_dict(context.user_data.get(_profile_key()))


def update_profile(context: ContextTypes.DEFAULT_TYPE, **updates: Any) -> UserPrefs:
    payload = get_user_prefs(context).to_dict()
    payload.update({key: value for key, value in updates.items() if value is not None})
    context.user_data[_profile_key()] = payload
    return UserPrefs.from_dict(payload)


def _message_target(update: Update):
    return update.callback_query.message if update.callback_query else update.message


async def _reply(
    update: Update,
    text: str,
    *,
    reply_markup: Any | None = None,
) -> None:
    await _message_target(update).reply_text(text, reply_markup=reply_markup)


async def _edit_or_reply(
    update: Update,
    text: str,
    *,
    reply_markup: Any | None = None,
) -> None:
    if update.callback_query:
        await update.callback_query.answer()
        await update.callback_query.edit_message_text(text, reply_markup=reply_markup)
        return
    await _reply(update, text, reply_markup=reply_markup)


def _format_minutes(value: float) -> str:
    if value < 1:
        return "<1 min"
    return f"{round(value):.0f} min"


def _truncate(text: str, limit: int = 220) -> str:
    compact = " ".join(text.split())
    if len(compact) <= limit:
        return compact
    return compact[: limit - 3].rstrip() + "..."


def _format_place(place: FoodPlace) -> str:
    lines = [place.name]
    if place.location:
        lines.append(f"Location: {place.location}")
    if place.category:
        lines.append(f"Category: {place.category}")
    if place.description:
        lines.append(f"Why it might fit: {_truncate(place.description)}")
    if place.url:
        lines.append(f"URL: {place.url}")
    return "\n".join(lines)


def _format_source(source_status: str) -> str:
    return {
        "live": "live scraper",
        "cached": "cached results",
        "seed": "bundled seed data",
        "unavailable": "no available data source",
    }.get(source_status, source_status)


def _current_selection_summary(context: ContextTypes.DEFAULT_TYPE) -> str:
    prefs = get_user_prefs(context)
    registry = _get_registry(context)
    lines = []
    live_location = context.user_data.get("live_location")
    selected_area_id = context.user_data.get("selected_area_id")

    if live_location:
        lines.append("Using your live Telegram location.")
    elif selected_area_id and selected_area_id in registry:
        lines.append(f"Using saved/manual area: {registry[selected_area_id].label}.")
    elif prefs.last_area_id and prefs.last_area_id in registry:
        lines.append(f"Last area on file: {registry[prefs.last_area_id].label}.")
    else:
        lines.append("No area is selected yet.")

    pace = context.user_data.get("pace_kmh") or prefs.pace_kmh
    walk_time = context.user_data.get("walk_time_minutes") or prefs.walk_time_minutes
    lines.append(f"Pace: {pace:.1f} km/h." if pace else "Pace: not set.")
    lines.append(
        f"Walking time: {walk_time} minutes." if walk_time else "Walking time: not set."
    )
    return "\n".join(lines)


def _nearest_area_id(
    registry: dict[str, Any], latitude: float, longitude: float
) -> str | None:
    nearest_id = None
    nearest_distance = None
    for area_id, area in registry.items():
        distance = haversine(latitude, longitude, area.latitude, area.longitude)
        if nearest_distance is None or distance < nearest_distance:
            nearest_id = area_id
            nearest_distance = distance
    return nearest_id


def _build_candidates(context: ContextTypes.DEFAULT_TYPE) -> list[AreaCandidate]:
    registry = _get_registry(context)
    prefs = get_user_prefs(context)
    pace = context.user_data.get("pace_kmh") or prefs.pace_kmh
    walk_time = context.user_data.get("walk_time_minutes") or prefs.walk_time_minutes
    live_location = context.user_data.get("live_location")
    selected_area_id = context.user_data.get("selected_area_id") or prefs.last_area_id

    if live_location and pace and walk_time:
        latitude, longitude = live_location
        candidates = []
        for area in registry.values():
            walkable, travel_minutes = locations_near(
                latitude,
                longitude,
                area.latitude,
                area.longitude,
                walk_time,
                pace,
            )
            distance = haversine(latitude, longitude, area.latitude, area.longitude)
            candidates.append(
                AreaCandidate(
                    area=area,
                    distance_km=distance,
                    travel_minutes=travel_minutes,
                    walkable=walkable,
                )
            )
        return sorted(candidates, key=lambda candidate: candidate.travel_minutes)

    if selected_area_id and selected_area_id in registry:
        area = registry[selected_area_id]
        return [
            AreaCandidate(
                area=area,
                distance_km=0.0,
                travel_minutes=0.0,
                walkable=True,
            )
        ]

    return []


def _result_keyboard(*rows: tuple[str, str]) -> InlineKeyboardMarkup:
    keyboard = [
        [InlineKeyboardButton(label, callback_data=callback_data)]
        for label, callback_data in rows
    ]
    return InlineKeyboardMarkup(keyboard)


def _pick_key(area_id: str, place: FoodPlace) -> str:
    return f"{area_id}|{place.url or place.name}|{place.name}"


def _remember_random_pick(
    context: ContextTypes.DEFAULT_TYPE,
    *,
    eligible_area_ids: list[str],
    area_id: str,
    result,
    place: FoodPlace,
) -> None:
    context.user_data["eligible_area_ids"] = eligible_area_ids
    context.user_data["last_area_items"] = [item.to_dict() for item in result.items]
    context.user_data["last_result_area_id"] = area_id
    context.user_data["last_result_source"] = result.source_status
    context.user_data["last_result_errors"] = result.errors
    context.user_data["last_store_name"] = place.name
    context.user_data["last_pick_key"] = _pick_key(area_id, place)

    seen_area_ids = list(context.user_data.get("seen_area_ids", []))
    if area_id not in seen_area_ids:
        seen_area_ids.append(area_id)
    context.user_data["seen_area_ids"] = seen_area_ids

    seen_pick_keys = list(context.user_data.get("seen_pick_keys", []))
    pick_key = _pick_key(area_id, place)
    if pick_key not in seen_pick_keys:
        seen_pick_keys.append(pick_key)
    context.user_data["seen_pick_keys"] = seen_pick_keys


async def _pick_random_place_from_area_pool(
    context: ContextTypes.DEFAULT_TYPE,
    eligible_area_ids: list[str],
    *,
    prefer_unseen_areas: bool,
    exclude_seen_picks: bool,
):
    provider = _get_provider(context)
    registry = _get_registry(context)

    seen_area_ids = set(context.user_data.get("seen_area_ids", []))
    seen_pick_keys = set(context.user_data.get("seen_pick_keys", []))
    unseen_area_ids = [area_id for area_id in eligible_area_ids if area_id not in seen_area_ids]
    seen_area_pool = [area_id for area_id in eligible_area_ids if area_id in seen_area_ids]
    area_groups: list[list[str]] = []

    if prefer_unseen_areas and unseen_area_ids:
        area_groups.append(unseen_area_ids)
        if seen_area_pool:
            area_groups.append(seen_area_pool)
    else:
        area_groups.append(list(eligible_area_ids))

    if not area_groups:
        area_groups.append(list(eligible_area_ids))

    combined_errors: list[str] = []
    attempted_keys: set[tuple[str, str]] = set()

    for area_group in area_groups:
        shuffled_area_ids = list(area_group)
        random.shuffle(shuffled_area_ids)
        options = []

        for area_id in shuffled_area_ids:
            result = await provider.get_food_places(registry[area_id])
            combined_errors.extend(result.errors)
            if not result.items:
                continue

            item_pool = list(result.items)
            if exclude_seen_picks:
                filtered_pool = [
                    item
                    for item in item_pool
                    if _pick_key(area_id, item) not in seen_pick_keys
                ]
                if filtered_pool:
                    item_pool = filtered_pool

            if not item_pool:
                continue

            dedupe_key = (area_id, result.source_status)
            if dedupe_key in attempted_keys:
                continue
            attempted_keys.add(dedupe_key)
            options.append((area_id, result, item_pool))

        if options:
            chosen_area_id, chosen_result, item_pool = random.choice(options)
            return chosen_area_id, chosen_result, random.choice(item_pool), combined_errors

    return None, None, None, combined_errors


def _area_picker_keyboard(
    registry: dict[str, Any], category: str, page: int
) -> InlineKeyboardMarkup:
    areas = areas_by_category(registry, category)
    start = page * AREA_PAGE_SIZE
    end = start + AREA_PAGE_SIZE
    selected = areas[start:end]
    keyboard = [
        [InlineKeyboardButton(area.label, callback_data=f"area:select:{area.id}")]
        for area in selected
    ]

    nav_row = []
    if page > 0:
        nav_row.append(
            InlineKeyboardButton("Previous", callback_data=f"area:page:{category}:{page - 1}")
        )
    if end < len(areas):
        nav_row.append(
            InlineKeyboardButton("Next", callback_data=f"area:page:{category}:{page + 1}")
        )
    if nav_row:
        keyboard.append(nav_row)

    alternate_category = "campus" if category == "mall" else "mall"
    keyboard.append(
        [
            InlineKeyboardButton(
                f"Switch to {alternate_category}s",
                callback_data=f"area:page:{alternate_category}:0",
            )
        ]
    )
    return InlineKeyboardMarkup(keyboard)


async def prompt_location_or_area(
    update: Update,
    context: ContextTypes.DEFAULT_TYPE,
    *,
    intro: str | None = None,
) -> int:
    prefs = get_user_prefs(context)
    registry = _get_registry(context)

    if intro:
        await _reply(update, intro, reply_markup=ReplyKeyboardRemove())

    location_keyboard = ReplyKeyboardMarkup(
        [[KeyboardButton("Share live location", request_location=True)]],
        resize_keyboard=True,
        one_time_keyboard=True,
    )
    await _reply(
        update,
        "Share your live Telegram location for the strongest nearby suggestions.",
        reply_markup=location_keyboard,
    )

    manual_buttons = [
        [
            InlineKeyboardButton("Browse malls", callback_data="area:page:mall:0"),
            InlineKeyboardButton("Browse campuses", callback_data="area:page:campus:0"),
        ]
    ]
    if prefs.last_area_id and prefs.last_area_id in registry:
        manual_buttons.append(
            [
                InlineKeyboardButton(
                    f"Use last area: {registry[prefs.last_area_id].label}",
                    callback_data="area:last",
                )
            ]
        )
    manual_buttons.append(
        [InlineKeyboardButton("Back to intents", callback_data="result:intents")]
    )

    await _reply(
        update,
        "No location? Pick an area manually instead.",
        reply_markup=InlineKeyboardMarkup(manual_buttons),
    )
    return LOCATION_OR_AREA


async def start(update: Update, context: ContextTypes.DEFAULT_TYPE) -> int:
    context.user_data["pending_intent"] = None
    await _reply(
        update,
        "Takko helps you decide where to eat around Singapore.\n\n"
        "Use /nearby for distance-aware area suggestions or /random for a single food pick.",
        reply_markup=ReplyKeyboardRemove(),
    )
    return await prompt_location_or_area(update, context)


async def help_command(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    await _reply(
        update,
        "Commands:\n"
        "/start - begin or restart the guided flow\n"
        "/nearby - find nearby food areas or eateries\n"
        "/random - get one food suggestion\n"
        "/profile - inspect your saved defaults\n"
        "/reset - clear saved defaults and session state",
        reply_markup=ReplyKeyboardRemove(),
    )


async def nearby_command(update: Update, context: ContextTypes.DEFAULT_TYPE) -> int:
    context.user_data["pending_intent"] = "nearby"
    return await prompt_location_or_area(
        update,
        context,
        intro="Nearby mode needs a live location or a selected area.",
    )


async def random_command(update: Update, context: ContextTypes.DEFAULT_TYPE) -> int:
    context.user_data["pending_intent"] = "random"
    return await prompt_location_or_area(
        update,
        context,
        intro="Random mode needs a live location or a selected area.",
    )


async def profile_command(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    prefs = get_user_prefs(context)
    registry = _get_registry(context)
    last_area = registry[prefs.last_area_id].label if prefs.last_area_id in registry else "Not set"
    pace = f"{prefs.pace_kmh:.1f} km/h" if prefs.pace_kmh else "Not set"
    walk_time = (
        f"{prefs.walk_time_minutes} minutes"
        if prefs.walk_time_minutes
        else "Not set"
    )
    await _reply(
        update,
        "Saved profile\n"
        f"- Pace: {pace}\n"
        f"- Walk time: {walk_time}\n"
        f"- Last area: {last_area}\n\n"
        "Use /reset if you want to clear this profile.",
        reply_markup=ReplyKeyboardRemove(),
    )


async def reset_command(update: Update, context: ContextTypes.DEFAULT_TYPE) -> int:
    context.user_data.clear()
    await _reply(
        update,
        "Your saved profile and current session were cleared.",
        reply_markup=ReplyKeyboardRemove(),
    )
    return ConversationHandler.END


async def handle_location(update: Update, context: ContextTypes.DEFAULT_TYPE) -> int:
    if not update.message or not update.message.location:
        await _reply(
            update,
            "I didn’t receive a live location. You can share it again or pick an area manually.",
        )
        return LOCATION_OR_AREA

    latitude = update.message.location.latitude
    longitude = update.message.location.longitude
    registry = _get_registry(context)
    nearest_area_id = _nearest_area_id(registry, latitude, longitude)

    context.user_data["live_location"] = (latitude, longitude)
    if nearest_area_id:
        context.user_data["selected_area_id"] = nearest_area_id
        update_profile(context, last_area_id=nearest_area_id)
        nearest_label = registry[nearest_area_id].label
        await _reply(
            update,
            f"Location received. Nearest tracked area: {nearest_label}.",
            reply_markup=ReplyKeyboardRemove(),
        )
    else:
        await _reply(
            update,
            "Location received. I’ll use it directly for nearby calculations.",
            reply_markup=ReplyKeyboardRemove(),
        )

    return await prompt_pace(update, context)


async def handle_area_navigation(
    update: Update, context: ContextTypes.DEFAULT_TYPE
) -> int:
    _, _, category, page = update.callback_query.data.split(":")
    registry = _get_registry(context)
    await _edit_or_reply(
        update,
        f"Choose a {category} area.",
        reply_markup=_area_picker_keyboard(registry, category, int(page)),
    )
    return LOCATION_OR_AREA


async def handle_use_last_area(
    update: Update, context: ContextTypes.DEFAULT_TYPE
) -> int:
    prefs = get_user_prefs(context)
    registry = _get_registry(context)
    if not prefs.last_area_id or prefs.last_area_id not in registry:
        await _edit_or_reply(
            update,
            "There isn’t a saved area yet. Share a location or browse an area manually.",
        )
        return LOCATION_OR_AREA

    context.user_data.pop("live_location", None)
    context.user_data["selected_area_id"] = prefs.last_area_id
    await _edit_or_reply(
        update,
        f"Using your saved area: {registry[prefs.last_area_id].label}.",
    )
    return await prompt_pace(update, context)


async def handle_area_selection(
    update: Update, context: ContextTypes.DEFAULT_TYPE
) -> int:
    area_id = update.callback_query.data.split(":")[-1]
    registry = _get_registry(context)
    if area_id not in registry:
        await _edit_or_reply(update, "That area is no longer available.")
        return LOCATION_OR_AREA

    context.user_data.pop("live_location", None)
    context.user_data["selected_area_id"] = area_id
    update_profile(context, last_area_id=area_id)
    await _edit_or_reply(update, f"Selected area: {registry[area_id].label}.")
    return await prompt_pace(update, context)


async def prompt_pace(update: Update, context: ContextTypes.DEFAULT_TYPE) -> int:
    prefs = get_user_prefs(context)
    keyboard = [
        [
            InlineKeyboardButton(
                f"{label} ({pace:.1f} km/h)", callback_data=f"pace:set:{pace}"
            )
        ]
        for label, pace in PACE_PRESETS
    ]
    if prefs.pace_kmh:
        keyboard.append(
            [
                InlineKeyboardButton(
                    f"Use saved pace ({prefs.pace_kmh:.1f} km/h)",
                    callback_data="pace:saved",
                )
            ]
        )

    await _reply(
        update,
        "How fast do you usually walk?\n\n"
        f"{_current_selection_summary(context)}",
        reply_markup=InlineKeyboardMarkup(keyboard),
    )
    return PACE


async def handle_pace_selection(
    update: Update, context: ContextTypes.DEFAULT_TYPE
) -> int:
    data = update.callback_query.data
    prefs = get_user_prefs(context)
    if data == "pace:saved":
        if prefs.pace_kmh is None:
            await _edit_or_reply(update, "No saved pace yet. Pick one of the presets.")
            return PACE
        pace_kmh = prefs.pace_kmh
    else:
        pace_kmh = float(data.split(":")[-1])

    context.user_data["pace_kmh"] = pace_kmh
    update_profile(context, pace_kmh=pace_kmh)
    await _edit_or_reply(update, f"Pace saved at {pace_kmh:.1f} km/h.")
    return await prompt_walk_time(update, context)


async def prompt_walk_time(update: Update, context: ContextTypes.DEFAULT_TYPE) -> int:
    prefs = get_user_prefs(context)
    keyboard = [
        [
            InlineKeyboardButton(
                f"{minutes} minutes", callback_data=f"walk:set:{minutes}"
            )
        ]
        for minutes in WALK_TIME_PRESETS
    ]
    if prefs.walk_time_minutes:
        keyboard.append(
            [
                InlineKeyboardButton(
                    f"Use saved walk time ({prefs.walk_time_minutes} min)",
                    callback_data="walk:saved",
                )
            ]
        )

    await _reply(
        update,
        "How long are you willing to walk today?",
        reply_markup=InlineKeyboardMarkup(keyboard),
    )
    return WALK_TIME


async def handle_walk_time_selection(
    update: Update, context: ContextTypes.DEFAULT_TYPE
) -> int:
    data = update.callback_query.data
    prefs = get_user_prefs(context)
    if data == "walk:saved":
        if prefs.walk_time_minutes is None:
            await _edit_or_reply(update, "No saved walk time yet. Pick a preset.")
            return WALK_TIME
        walk_time_minutes = prefs.walk_time_minutes
    else:
        walk_time_minutes = int(data.split(":")[-1])

    context.user_data["walk_time_minutes"] = walk_time_minutes
    update_profile(context, walk_time_minutes=walk_time_minutes)
    pending_intent = context.user_data.pop("pending_intent", None)
    await _edit_or_reply(update, f"Walking time saved at {walk_time_minutes} minutes.")
    if pending_intent:
        return await execute_intent(update, context, pending_intent)
    return await prompt_intent(update, context)


async def prompt_intent(update: Update, context: ContextTypes.DEFAULT_TYPE) -> int:
    keyboard = InlineKeyboardMarkup(
        [
            [InlineKeyboardButton("Nearby food", callback_data="intent:nearby")],
            [InlineKeyboardButton("Spin the wheel", callback_data="intent:random")],
            [InlineKeyboardButton("I have 15 minutes", callback_data="intent:fast15")],
            [InlineKeyboardButton("Change area", callback_data="result:change_area")],
        ]
    )
    await _reply(
        update,
        "What should Takko do next?\n\n" + _current_selection_summary(context),
        reply_markup=keyboard,
    )
    return INTENT


async def handle_intent_selection(
    update: Update, context: ContextTypes.DEFAULT_TYPE
) -> int:
    intent = update.callback_query.data.split(":")[-1]
    return await execute_intent(update, context, intent)


async def execute_intent(
    update: Update, context: ContextTypes.DEFAULT_TYPE, intent: str
) -> int:
    prefs = get_user_prefs(context)
    if intent == "fast15":
        context.user_data["walk_time_minutes"] = FAST_LUNCH_MINUTES
        update_profile(context, walk_time_minutes=FAST_LUNCH_MINUTES)
        if not context.user_data.get("pace_kmh"):
            context.user_data["pace_kmh"] = prefs.pace_kmh or DEFAULT_FAST_PACE_KMH
        if not context.user_data.get("selected_area_id") and not context.user_data.get(
            "live_location"
        ):
            if prefs.last_area_id:
                context.user_data["selected_area_id"] = prefs.last_area_id
            else:
                return await prompt_location_or_area(
                    update,
                    context,
                    intro="Fast lunch mode needs a live location or a saved area.",
                )
        intent = "random"

    context.user_data["last_intent"] = intent
    if intent == "nearby":
        return await run_nearby_flow(update, context)
    if intent == "random":
        return await run_random_flow(update, context)

    await _edit_or_reply(update, "That action is not supported.")
    return INTENT


def _candidate_lookup(context: ContextTypes.DEFAULT_TYPE) -> dict[str, AreaCandidate]:
    return {candidate.area.id: candidate for candidate in _build_candidates(context)}


async def run_nearby_flow(update: Update, context: ContextTypes.DEFAULT_TYPE) -> int:
    provider = _get_provider(context)
    registry = _get_registry(context)
    live_location = context.user_data.get("live_location")
    candidates = _build_candidates(context)

    if live_location:
        walkable = [candidate for candidate in candidates if candidate.walkable]
        alternatives = [candidate for candidate in candidates if not candidate.walkable][:3]
        context.user_data["eligible_area_ids"] = [candidate.area.id for candidate in walkable]

        if walkable:
            lines = ["Walkable food areas"]
            for candidate in walkable[:5]:
                lines.append(
                    f"- {candidate.area.label}: {candidate.distance_km:.2f} km, "
                    f"about {_format_minutes(candidate.travel_minutes)}"
                )
            if alternatives:
                lines.append("")
                lines.append("Closest alternatives outside your radius")
                for candidate in alternatives:
                    lines.append(
                        f"- {candidate.area.label}: {candidate.distance_km:.2f} km, "
                        f"about {_format_minutes(candidate.travel_minutes)}"
                    )
        else:
            lines = [
                "Nothing is within your current walking radius.",
                "",
                "Closest alternatives",
            ]
            for candidate in candidates[:3]:
                lines.append(
                    f"- {candidate.area.label}: {candidate.distance_km:.2f} km, "
                    f"about {_format_minutes(candidate.travel_minutes)}"
                )

        await _edit_or_reply(
            update,
            "\n".join(lines),
            reply_markup=_result_keyboard(
                ("Broaden radius by 10 minutes", "result:broaden"),
                ("Spin the wheel", "intent:random"),
                ("Choose another area", "result:change_area"),
            ),
        )
        return RESULTS

    if not candidates:
        return await prompt_location_or_area(
            update,
            context,
            intro="Nearby mode needs a selected area when live location is not available.",
        )

    area = registry[candidates[0].area.id]
    result = await provider.get_food_places(area)
    context.user_data["last_area_items"] = [item.to_dict() for item in result.items]
    context.user_data["last_result_area_id"] = area.id
    context.user_data["last_result_source"] = result.source_status
    context.user_data["last_result_errors"] = result.errors

    lines = [
        f"Eateries in {area.label}",
        f"Source: {_format_source(result.source_status)}",
    ]
    if result.errors:
        lines.append(f"Note: {_truncate(result.errors[0], limit=180)}")
    if result.items:
        for item in result.items[:5]:
            lines.append("")
            lines.append(_format_place(item))
    else:
        lines.append("No eateries were available for this area.")

    await _edit_or_reply(
        update,
        "\n".join(lines),
        reply_markup=_result_keyboard(
            ("Spin the wheel here", "intent:random"),
            ("Show shortlist", "result:shortlist"),
            ("Choose another area", "result:change_area"),
        ),
    )
    return RESULTS


async def run_random_flow(update: Update, context: ContextTypes.DEFAULT_TYPE) -> int:
    registry = _get_registry(context)
    candidates = _build_candidates(context)
    live_location = context.user_data.get("live_location")

    if live_location:
        eligible_area_ids = [candidate.area.id for candidate in candidates if candidate.walkable]
    else:
        eligible_area_ids = [candidate.area.id for candidate in candidates]

    if not eligible_area_ids:
        await _edit_or_reply(
            update,
            "I couldn’t find any walkable areas with your current settings.",
            reply_markup=_result_keyboard(
                ("Broaden radius by 10 minutes", "result:broaden"),
                ("Choose another area", "result:change_area"),
            ),
        )
        return RESULTS

    context.user_data["seen_area_ids"] = []
    context.user_data["seen_pick_keys"] = []
    chosen_area_id, chosen_result, place, combined_errors = await _pick_random_place_from_area_pool(
        context,
        eligible_area_ids,
        prefer_unseen_areas=False,
        exclude_seen_picks=False,
    )

    if chosen_result is None or chosen_area_id is None or place is None:
        await _edit_or_reply(
            update,
            "I couldn’t load any eateries from the available areas right now.\n"
            + ("\n".join(combined_errors[:3]) if combined_errors else ""),
            reply_markup=_result_keyboard(
                ("Choose another area", "result:change_area"),
                ("Back to intents", "result:intents"),
            ),
        )
        return RESULTS

    _remember_random_pick(
        context,
        eligible_area_ids=eligible_area_ids,
        area_id=chosen_area_id,
        result=chosen_result,
        place=place,
    )

    lines = [
        "Try this spot",
        _format_place(place),
        "",
        f"Area: {registry[chosen_area_id].label}",
        f"Source: {_format_source(chosen_result.source_status)}",
    ]
    candidate_map = _candidate_lookup(context)
    if chosen_area_id in candidate_map and live_location:
        candidate = candidate_map[chosen_area_id]
        lines.append(
            f"Travel estimate: {candidate.distance_km:.2f} km, about {_format_minutes(candidate.travel_minutes)}"
        )
    if chosen_result.errors:
        lines.append(f"Note: {_truncate(chosen_result.errors[0], limit=180)}")

    rows = [
        ("Reroll", "result:reroll"),
        ("Show shortlist", "result:shortlist"),
        ("Nearby areas", "intent:nearby"),
    ]
    if live_location:
        rows.append(("Broaden radius by 10 minutes", "result:broaden"))
    rows.append(("Choose another area", "result:change_area"))

    await _edit_or_reply(update, "\n".join(lines), reply_markup=_result_keyboard(*rows))
    return RESULTS


def _rehydrate_places(raw_items: list[dict[str, str]]) -> list[FoodPlace]:
    return [FoodPlace.from_raw(item) for item in raw_items]


async def handle_result_action(
    update: Update, context: ContextTypes.DEFAULT_TYPE
) -> int:
    action = update.callback_query.data

    if action == "result:change_area":
        return await prompt_location_or_area(
            update,
            context,
            intro="Pick a new area or share a new live location.",
        )

    if action == "result:intents":
        return await prompt_intent(update, context)

    if action == "result:broaden":
        current_walk_time = context.user_data.get("walk_time_minutes") or FAST_LUNCH_MINUTES
        broadened = current_walk_time + 10
        context.user_data["walk_time_minutes"] = broadened
        await _edit_or_reply(update, f"Walking radius increased to {broadened} minutes.")
        return await execute_intent(
            update, context, context.user_data.get("last_intent", "nearby")
        )

    if action == "result:reroll":
        eligible_area_ids = context.user_data.get("eligible_area_ids", [])
        if not eligible_area_ids:
            return await run_random_flow(update, context)

        chosen_area_id, chosen_result, place, combined_errors = await _pick_random_place_from_area_pool(
            context,
            eligible_area_ids,
            prefer_unseen_areas=True,
            exclude_seen_picks=True,
        )
        if chosen_result is None or chosen_area_id is None or place is None:
            context.user_data["seen_area_ids"] = []
            context.user_data["seen_pick_keys"] = []
            chosen_area_id, chosen_result, place, combined_errors = await _pick_random_place_from_area_pool(
                context,
                eligible_area_ids,
                prefer_unseen_areas=False,
                exclude_seen_picks=False,
            )
        if chosen_result is None or chosen_area_id is None or place is None:
            return await run_random_flow(update, context)

        _remember_random_pick(
            context,
            eligible_area_ids=eligible_area_ids,
            area_id=chosen_area_id,
            result=chosen_result,
            place=place,
        )
        registry = _get_registry(context)
        source_status = context.user_data.get("last_result_source", "cached")
        errors = context.user_data.get("last_result_errors", [])
        lines = [
            "Reroll result",
            _format_place(place),
            "",
            f"Area: {registry[chosen_area_id].label}",
            f"Source: {_format_source(source_status)}",
        ]
        if errors:
            lines.append(f"Note: {_truncate(errors[0], limit=180)}")
        elif combined_errors:
            lines.append(f"Note: {_truncate(combined_errors[0], limit=180)}")
        await _edit_or_reply(
            update,
            "\n".join(lines),
            reply_markup=_result_keyboard(
                ("Reroll again", "result:reroll"),
                ("Show shortlist", "result:shortlist"),
                ("Nearby areas", "intent:nearby"),
                ("Choose another area", "result:change_area"),
            ),
        )
        return RESULTS

    if action == "result:shortlist":
        raw_items = context.user_data.get("last_area_items", [])
        if not raw_items:
            await _edit_or_reply(update, "No shortlist is available yet.")
            return RESULTS

        places = _rehydrate_places(raw_items)
        shortlist = random.sample(places, k=min(3, len(places)))
        lines = ["Shortlist"]
        for index, place in enumerate(shortlist, start=1):
            lines.append("")
            lines.append(f"{index}. {_format_place(place)}")

        await _edit_or_reply(
            update,
            "\n".join(lines),
            reply_markup=_result_keyboard(
                ("Reroll", "result:reroll"),
                ("Nearby areas", "intent:nearby"),
                ("Choose another area", "result:change_area"),
            ),
        )
        return RESULTS

    await _edit_or_reply(update, "That action is not available.")
    return RESULTS


async def handle_error(update: object, context: ContextTypes.DEFAULT_TYPE) -> None:
    logger.exception("Unhandled Telegram bot error", exc_info=context.error)
    if isinstance(update, Update):
        try:
            await _reply(
                update,
                "Something went wrong while processing that request. "
                "Please try again or use /start to restart the flow.",
                reply_markup=ReplyKeyboardRemove(),
            )
        except Exception:  # pragma: no cover - defensive path
            logger.exception("Failed to notify user about the handler error")


def build_application(
    settings: Settings,
    *,
    registry: dict[str, Any] | None = None,
    provider: FoodProvider | None = None,
) -> Application:
    registry = registry or load_area_registry()
    persistence = PicklePersistence(filepath=str(settings.bot_state_path))
    application = ApplicationBuilder().token(settings.bot_token).persistence(persistence).build()
    application.bot_data["settings"] = settings
    application.bot_data["registry"] = registry
    application.bot_data["food_provider"] = provider or FoodProvider(settings)

    conversation = ConversationHandler(
        entry_points=[
            CommandHandler("start", start),
            CommandHandler("nearby", nearby_command),
            CommandHandler("random", random_command),
        ],
        states={
            LOCATION_OR_AREA: [
                MessageHandler(filters.LOCATION, handle_location),
                CallbackQueryHandler(handle_area_navigation, pattern=r"^area:page:"),
                CallbackQueryHandler(handle_use_last_area, pattern=r"^area:last$"),
                CallbackQueryHandler(handle_area_selection, pattern=r"^area:select:"),
                CallbackQueryHandler(handle_result_action, pattern=r"^result:intents$"),
            ],
            PACE: [
                CallbackQueryHandler(handle_pace_selection, pattern=r"^pace:"),
            ],
            WALK_TIME: [
                CallbackQueryHandler(handle_walk_time_selection, pattern=r"^walk:"),
            ],
            INTENT: [
                CallbackQueryHandler(handle_intent_selection, pattern=r"^intent:"),
                CallbackQueryHandler(handle_result_action, pattern=r"^result:change_area$"),
            ],
            RESULTS: [
                CallbackQueryHandler(handle_intent_selection, pattern=r"^intent:"),
                CallbackQueryHandler(handle_result_action, pattern=r"^result:"),
            ],
        },
        fallbacks=[
            CommandHandler("start", start),
            CommandHandler("nearby", nearby_command),
            CommandHandler("random", random_command),
            CommandHandler("reset", reset_command),
        ],
        allow_reentry=True,
    )

    application.add_handler(CommandHandler("help", help_command))
    application.add_handler(CommandHandler("profile", profile_command))
    application.add_handler(CommandHandler("reset", reset_command))
    application.add_handler(conversation)
    application.add_error_handler(handle_error)
    return application


def main() -> None:
    settings = load_settings()
    registry = load_area_registry()
    asyncio.run(validate_runtime(settings, registry))
    application = build_application(settings, registry=registry)
    logger.info("Takko bot is polling")
    application.run_polling()


if __name__ == "__main__":
    main()
