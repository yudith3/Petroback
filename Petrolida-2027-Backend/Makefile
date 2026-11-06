.PHONY: build check db-down db-reset db-up dev fmt run test tidy vet

run:
	go run .

dev:
	air -c .air.toml

build:
	go build -o bin/api .

test:
	go test ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

check:
	go test ./...
	go vet ./...
	go build ./...

tidy:
	go mod tidy

db-up:
	docker compose up -d postgres

db-down:
	docker compose down

db-reset:
	docker compose down -v
	docker compose up -d postgres
