.PHONY: api-test api-run db-up db-down

api-test:
	cd services/api && go test ./...

api-run:
	cd services/api && go run ./cmd/server

db-up:
	docker compose -f deploy/compose.yaml up -d postgres

db-down:
	docker compose -f deploy/compose.yaml down
