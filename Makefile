all:test

test:manage.py
	@echo "serving frontend by executing manage.py..."
	@python3 manage.py runserver

config:
	@echo "installing dependencies..."
	@sudo apt update && sudo apt upgrade && sudo apt autoremove
	@sudo apt install postgresql postgresql-contrib
	@pip install django djangorestframework scrapy beautifulsoup4 psycopg2-binary
	@echo "starting postgresql database..."
	@sudo service postgresql start
	@systemctl list-units --type=service | grep postgresql
	@echo "initializing database cluster..."
	@sudo service postgresql initdb
	@echo "configuing postgresql to start on boot..."
	@sudo systemctl enable postgresql
	@echo "configuration complete"