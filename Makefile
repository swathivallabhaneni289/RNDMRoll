.PHONY: run dev test test-integration migrate-up migrate-down migrate-new db-create

run:
	go run ./cmd/api

dev:
	set -a && . ./.env && set +a && go run ./cmd/api

test:
	go test ./...

test-integration:
	TEST_DATABASE_URL="$${TEST_DATABASE_URL:-postgres://localhost:5432/rndmroll_test?sslmode=disable}" go test ./internal/httpapi -v -count=1

migrate-up:
	migrate -path migrations -database "$$DATABASE_URL" up

migrate-down:
	migrate -path migrations -database "$$DATABASE_URL" down 1

migrate-new:
	migrate create -ext sql -dir migrations -seq $$NAME

db-create:
	createdb rndmroll_dev || true
	createdb rndmroll_test || true
