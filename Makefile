all:test

test:mock.py
	@echo "(1/1) running testing file at mock.py..."
	@python3 mock.py

config:
	@echo "(0/4) installing dependencies..."
	@echo "(1/4) installing django..."
	@echo "(2/4) installing djangorestframework..."
	@echo "(3/4) installing scrapy..."
	@echo "(4/4) installing beautifulsoup4..."
	@pip install django djangorestframework scrapy beautifulsoup4