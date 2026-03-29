API_DIR := ./api

.PHONY: test run ingest ingest-artist

test:
	cd $(API_DIR) && go test ./...

run:
	cd $(API_DIR) && go run .

ingest:
	cd $(API_DIR) && go run ./cmd/ingest -all=true

ingest-artist:
	cd $(API_DIR) && go run ./cmd/ingest -artist "$(ARTIST)"
