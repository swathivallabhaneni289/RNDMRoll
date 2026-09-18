.PHONY: run test migrate-up migrate-down migrate-new db-create

run:
	go run ./cmd/api

test:
	go test ./...

migrate-up:
	migrate -path migrations -database "$$DATABASE_URL" up

migrate-down:
	migrate -path migrations -database "$$DATABASE_URL" down 1

migrate-new:
	migrate create -ext sql -dir migrations -seq $$NAME

db-create:
	createdb rndmroll_dev || true
	createdb rndmroll_test || true
