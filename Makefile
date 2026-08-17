.PHONY: api-test api-check api-run db-up db-down db-migrate db-seed db-reset

DATABASE_URL ?= postgres://rivo:rivo_dev@localhost:5432/rivo?sslmode=disable

api-test:
	cd services/api && go test ./...

# Run before opening a pull request.
api-check:
	cd services/api && gofmt -l . && go vet ./... && go test ./...

api-run:
	cd services/api && go run ./cmd/server

db-up:
	docker compose -f deploy/compose.yaml up -d postgres

db-down:
	docker compose -f deploy/compose.yaml down

# Compose applies migrations automatically the first time the volume is created.
# These targets are for an existing database.
db-migrate:
	for migration in database/migrations/*.sql; do \
		psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f "$$migration" || exit 1; \
	done

db-seed:
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f database/seed/dev_seed.sql

db-reset: db-migrate db-seed
