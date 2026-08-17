.PHONY: api-test api-check api-run db-up db-down db-migrate db-seed db-verify db-reset smoke lint-ci check

DATABASE_URL ?= postgres://rivo:rivo_dev@localhost:5432/rivo?sslmode=disable

api-test:
	cd services/api && go test ./...

# Run before opening a pull request. Mirrors the Go API job in CI.
api-check:
	cd services/api && gofmt -l . && go vet ./... && go test -race ./...

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

# Asserts that the moderation invariants really are enforced by the schema. Runs in
# a transaction that is rolled back, so it is safe against a seeded database.
db-verify:
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f database/tests/constraints.sql

db-reset: db-migrate db-seed

# Starts the real binary against a real database and drives it over HTTP.
smoke:
	DATABASE_URL="$(DATABASE_URL)" scripts/smoke.sh

# A broken workflow file does not run, so CI cannot tell you it is broken.
# Check it here instead.
lint-ci:
	go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.7
	shellcheck scripts/*.sh

# Everything CI runs, against a database you have already started with db-up.
check: lint-ci api-check db-verify smoke
