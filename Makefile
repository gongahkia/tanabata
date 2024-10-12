all: test

test: manage.py
	@clear
	@echo "serving frontend by executing manage.py..."
	@python3 manage.py runserver

config:
	@clear
	@echo "installing dependencies..."
	@sudo apt update && sudo apt upgrade -y && sudo apt autoremove -y
	@sudo apt install -y postgresql postgresql-contrib firefox
	@pip install django djangorestframework scrapy beautifulsoup4 psycopg2-binary webdriver_manager selenium scrapy-splash requests_html lxml[html_clean] aiohttp
	@sudo snap install docker
	@echo "starting postgresql database..."
	@sudo service postgresql start
	@echo "checking postgresql service status..."
	@systemctl list-units --type=service | grep postgresql || echo "postgreSQL service not found"
	@echo "initializing database cluster..."
	@sudo service postgresql initdb || echo "database cluster already initialized"
	@echo "configuring postgresql to start on boot..."
	@sudo systemctl enable postgresql
	@echo "configuration complete"

mock:
	@clear
	@echo "starting up postgresql cli tool..."
	@sudo -u postgres psql -f mock.sql
	@echo "created database and user..."
	@echo "\q to quit"
	@echo "checking postgresql server status..."
	@pg_isready -U your_db_user
	@echo "\dt --> lists tables"
	@echo "\d {table_name} --> list table of the specified name"
	@echo "\q --> quit"
	@psql -h localhost -U user_gongahkia -d tako_db

migrate:manage.py
	@python3 manage.py makemigrations
	@python3 manage.py migrate
	@python3 manage.py runserver