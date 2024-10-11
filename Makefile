all:test

test:mock.py
	@echo "(1/1) running testing file at mock.py..."
	@python3 mock.py

config:
	@echo "(1/1) installing dependencies..."
	@pip install django djangorestframework