import asyncio
from pathlib import Path
from types import SimpleNamespace

import pytest
from telegram import InlineKeyboardMarkup

import bot.bot as bot_module
import bot.config as config_module
from bot.area_registry import load_area_registry
from bot.config import Settings, load_settings
from bot.models import AreaDefinition, FoodPlace, ScraperResult


class DummyMessage:
    def __init__(self, location=None):
        self.location = location
        self.replies = []

    async def reply_text(self, text, reply_markup=None):
        self.replies.append({"text": text, "reply_markup": reply_markup})


class DummyCallbackQuery:
    def __init__(self, data, message=None):
        self.data = data
        self.message = message or DummyMessage()
        self.edits = []
        self.answered = False

    async def answer(self):
        self.answered = True

    async def edit_message_text(self, text, reply_markup=None):
        self.edits.append({"text": text, "reply_markup": reply_markup})


class DummyApplication:
    def __init__(self, bot_data=None):
        self.bot_data = bot_data or {}


class DummyContext:
    def __init__(self, user_data=None, bot_data=None):
        self.user_data = user_data or {}
        self.application = DummyApplication(bot_data=bot_data)
        self.error = None


def make_stub_registry():
    return {
        "nearby_mall": AreaDefinition(
            id="nearby_mall",
            label="Nearby Mall",
            category="mall",
            latitude=1.3001,
            longitude=103.8001,
            scraper_module="smu_shakespeare",
            source_urls=("https://example.com/nearby",),
            seed_file=None,
        ),
        "far_mall": AreaDefinition(
            id="far_mall",
            label="Far Mall",
            category="mall",
            latitude=1.45,
            longitude=103.95,
            scraper_module="smu_shakespeare",
            source_urls=("https://example.com/far",),
            seed_file=None,
        ),
    }


class StubProvider:
    async def get_food_places(self, area):
        return ScraperResult(
            items=[
                FoodPlace(
                    name=f"{area.label} Cafe",
                    location=area.label,
                    description="Solid lunch option",
                    category="Cafe",
                    url="https://example.com",
                )
            ],
            source_status="seed",
            errors=["live failed"],
        )


def test_load_settings_requires_bot_token(monkeypatch):
    monkeypatch.delenv("BOT_TOKEN", raising=False)

    with pytest.raises(RuntimeError, match="BOT_TOKEN"):
        load_settings()


def test_validate_runtime_creates_state_dirs(tmp_path, monkeypatch):
    settings = Settings(
        bot_token="123:test-token",
        bot_state_path=tmp_path / ".state" / "bot_state.pkl",
    )

    async def fake_validate_playwright_browser():
        return None

    monkeypatch.setattr(config_module, "validate_playwright_browser", fake_validate_playwright_browser)
    asyncio.run(config_module.validate_runtime(settings, load_area_registry()))

    assert settings.state_dir.exists()
    assert settings.cache_dir.exists()


def test_build_application_smoke(tmp_path):
    settings = Settings(
        bot_token="123:test-token",
        bot_state_path=tmp_path / ".state" / "bot_state.pkl",
    )
    app = bot_module.build_application(
        settings,
        registry=load_area_registry(),
        provider=StubProvider(),
    )

    assert "registry" in app.bot_data
    assert "food_provider" in app.bot_data


def test_walk_time_selection_preserves_pending_random_intent(monkeypatch):
    update = SimpleNamespace(
        callback_query=DummyCallbackQuery("walk:set:15"),
        message=None,
    )
    context = DummyContext(user_data={"pending_intent": "random", "profile": {}})

    async def fake_execute_intent(update, context, intent):
        context.user_data["executed_intent"] = intent
        return 999

    monkeypatch.setattr(bot_module, "execute_intent", fake_execute_intent)

    state = asyncio.run(bot_module.handle_walk_time_selection(update, context))

    assert state == 999
    assert context.user_data["executed_intent"] == "random"
    assert context.user_data["walk_time_minutes"] == 15


def test_prompt_location_or_area_offers_saved_area():
    registry = make_stub_registry()
    update = SimpleNamespace(message=DummyMessage(), callback_query=None)
    context = DummyContext(
        user_data={"profile": {"last_area_id": "nearby_mall"}},
        bot_data={"registry": registry},
    )

    state = asyncio.run(bot_module.prompt_location_or_area(update, context))

    assert state == bot_module.LOCATION_OR_AREA
    reply_markup = update.message.replies[-1]["reply_markup"]
    assert isinstance(reply_markup, InlineKeyboardMarkup)
    callback_data = [
        button.callback_data
        for row in reply_markup.inline_keyboard
        for button in row
    ]
    assert "area:last" in callback_data


def test_run_random_flow_uses_manual_area_and_provider():
    registry = make_stub_registry()
    update = SimpleNamespace(
        callback_query=DummyCallbackQuery("intent:random"),
        message=None,
    )
    context = DummyContext(
        user_data={
            "profile": {},
            "selected_area_id": "nearby_mall",
            "pace_kmh": 5.5,
            "walk_time_minutes": 15,
        },
        bot_data={"registry": registry, "food_provider": StubProvider()},
    )

    state = asyncio.run(bot_module.run_random_flow(update, context))

    assert state == bot_module.RESULTS
    assert "Try this spot" in update.callback_query.edits[-1]["text"]
    assert "Source: bundled seed data" in update.callback_query.edits[-1]["text"]


def test_reroll_prefers_an_unseen_area_from_the_eligible_pool(monkeypatch):
    registry = make_stub_registry()
    update = SimpleNamespace(
        callback_query=DummyCallbackQuery("result:reroll"),
        message=None,
    )
    context = DummyContext(
        user_data={
            "profile": {},
            "eligible_area_ids": ["nearby_mall", "far_mall"],
            "seen_area_ids": ["nearby_mall"],
            "seen_pick_keys": ["nearby_mall|https://example.com|Nearby Mall Cafe"],
            "last_result_source": "seed",
            "last_result_errors": ["live failed"],
        },
        bot_data={"registry": registry, "food_provider": StubProvider()},
    )

    monkeypatch.setattr(bot_module.random, "shuffle", lambda seq: None)
    monkeypatch.setattr(bot_module.random, "choice", lambda seq: seq[0])

    state = asyncio.run(bot_module.handle_result_action(update, context))

    assert state == bot_module.RESULTS
    assert "Area: Far Mall" in update.callback_query.edits[-1]["text"]
    assert context.user_data["last_result_area_id"] == "far_mall"
    assert set(context.user_data["seen_area_ids"]) == {"nearby_mall", "far_mall"}


def test_run_nearby_flow_offers_broaden_when_nothing_is_walkable():
    registry = make_stub_registry()
    update = SimpleNamespace(
        callback_query=DummyCallbackQuery("intent:nearby"),
        message=None,
    )
    context = DummyContext(
        user_data={
            "profile": {},
            "live_location": (1.20, 103.60),
            "pace_kmh": 4.5,
            "walk_time_minutes": 5,
        },
        bot_data={"registry": registry, "food_provider": StubProvider()},
    )

    state = asyncio.run(bot_module.run_nearby_flow(update, context))

    assert state == bot_module.RESULTS
    assert "Nothing is within your current walking radius." in update.callback_query.edits[-1]["text"]
    reply_markup = update.callback_query.edits[-1]["reply_markup"]
    callback_data = [
        button.callback_data
        for row in reply_markup.inline_keyboard
        for button in row
    ]
    assert "result:broaden" in callback_data
