API_DIR := ./api

.PHONY: test run ingest ingest-artist catalog-backup catalog-export

test:
	cd $(API_DIR) && go test ./...

run:
	cd $(API_DIR) && go run .

ingest:
	cd $(API_DIR) && go run ./cmd/ingest -bootstrap=true -all=true

ingest-artist:
	cd $(API_DIR) && go run ./cmd/ingest -artist "$(ARTIST)"

catalog-backup:
	cd $(API_DIR) && go run ./cmd/catalog -backup data/catalog.backup.sqlite

catalog-export:
	cd $(API_DIR) && go run ./cmd/catalog -export data/catalog.export.json
