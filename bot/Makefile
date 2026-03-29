PYTHON ?= python3.12

.PHONY: install install-playwright run test

install:
	$(PYTHON) -m pip install -r requirements-dev.txt

install-playwright:
	$(PYTHON) -m playwright install chromium

run:
	$(PYTHON) bot.py

test:
	PYTHONPATH=.. $(PYTHON) -m pytest -q ../tests
