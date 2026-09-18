.PHONY: run dev test test-integration test-all-integration migrate-up migrate-down migrate-new db-create

run:
	go run ./cmd/api

dev:
	set -a && . ./.env && set +a && go run ./cmd/api

test:
	go test ./...

test-integration:
	TEST_DATABASE_URL="$${TEST_DATABASE_URL:-postgres://localhost:5432/rndmroll_test?sslmode=disable}" go test ./internal/httpapi -v -count=1

# test-all-integration runs the full module test suite against a real test
# database with -p 1 (packages run one at a time). internal/httpapi and
# internal/store/postgres both truncate the shared `users` table between
# tests; go test's default package-level parallelism (-p, defaulting to
# GOMAXPROCS) can run those two packages' test binaries concurrently
# against the same TEST_DATABASE_URL, so a truncate from one package's
# cleanup can wipe rows a concurrently-running test in the other package
# still needs. -p 1 removes that race. Plain `go test ./...` (the `test`
# target above) stays safe because it never has TEST_DATABASE_URL set, so
# every DB-touching test in both packages skips.
test-all-integration:
	TEST_DATABASE_URL="$${TEST_DATABASE_URL:-postgres://localhost:5432/rndmroll_test?sslmode=disable}" go test ./... -p 1 -count=1

migrate-up:
	migrate -path migrations -database "$$DATABASE_URL" up

migrate-down:
	migrate -path migrations -database "$$DATABASE_URL" down 1

migrate-new:
	migrate create -ext sql -dir migrations -seq $$NAME

db-create:
	createdb rndmroll_dev || true
	createdb rndmroll_test || true
