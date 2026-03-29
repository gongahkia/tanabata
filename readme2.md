# Takko Bot Notes

This file documents the current Telegram bot implementation without modifying the original `README.md`.

## What Changed

The bot runtime under `bot/` now centers on a validated canonical area registry, a normalized food-data contract, and a fallback provider chain:

1. Live scraper
2. Fresh cached result
3. Bundled seed data from `scrapers/output/`

The active Telegram flow is now state-driven instead of one large handler script. It supports:

- `/start`
- `/nearby`
- `/random`
- `/profile`
- `/reset`

It also supports:

- live Telegram location
- manual area selection
- saved pace and walking-time defaults
- rerolling within the same eligible area set
- shortlist display
- broaden-radius fallback

## Runtime Layout

Key files:

- `bot/bot.py`: Telegram flow, handlers, persistence wiring, error handling
- `bot/area_registry.py`: canonical area definitions and validation
- `bot/models.py`: normalized types for areas, food items, scraper results, and user preferences
- `bot/food_provider.py`: live/cache/seed data provider chain
- `bot/config.py`: environment loading and runtime validation
- `bot/geolocation.py`: distance and walking-time calculations
- `tests/`: deterministic automated coverage

The old split metadata files were removed. `bot/area_registry.py` is now the single source of truth for supported areas.

## Environment

Required environment variables:

- `BOT_TOKEN`: Telegram bot token

Optional environment variables:

- `SCRAPER_TIMEOUT_SECONDS`: defaults to `20`
- `SCRAPER_CACHE_TTL_MINUTES`: defaults to `720`
- `BOT_STATE_PATH`: defaults to `./.state/bot_state.pkl`

## Local Setup

Recommended local Python version: `3.12`

Example setup:

```bash
cd bot
python3.12 -m venv .venv
source .venv/bin/activate
python -m pip install -r requirements-dev.txt
python -m playwright install chromium
```

Run the bot:

```bash
cd bot
python bot.py
```

## Testing

Run the test suite from the repo root:

```bash
PYTHONPATH=. python -m pytest -q tests
```

Or from `bot/`:

```bash
make test
```

The tests cover:

- area-registry validation
- normalization of scraper payloads
- provider fallback behavior
- saved-profile and conversation state behavior
- startup/build smoke paths

Live website scraping is intentionally excluded from CI.

## Deployment Notes

The polling bot is configured as a worker process:

```text
worker: python3 bot.py
```

Deployment checklist:

1. Set `BOT_TOKEN`
2. Install runtime dependencies from `bot/requirements.txt`
3. Install Playwright Chromium with `python -m playwright install chromium`
4. Ensure the process can write to `BOT_STATE_PATH` and its cache directory

## Troubleshooting

If startup fails:

- Missing `BOT_TOKEN`: set the token in the environment or `.env`
- Missing Playwright browser: run `python -m playwright install chromium`
- Registry validation error: inspect `bot/area_registry.py` for mismatched scraper modules or seed files
- Empty results from live scraping: Takko should fall back to cache or bundled seed data; if it does not, inspect `bot/food_provider.py` logs

If recommendations look stale:

- reduce `SCRAPER_CACHE_TTL_MINUTES`
- clear `.state/cache/`

## Current Limits

The current implementation intentionally does not ship:

- proactive scheduling/reminders
- reliable `open now` filtering
- structured dietary filters
- favourites/history or group decision mode

Those remain phase-two features after core runtime stability.
