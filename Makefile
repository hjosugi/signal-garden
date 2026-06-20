SHELL := /bin/sh
BINARY := bin/signal-garden

.PHONY: run build test vet fmt-check check smoke demo-data refresh docker-up docker-down

run:
	go run ./cmd/server

build:
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(BINARY) ./cmd/server

test:
	go test ./...

vet:
	go vet ./...

fmt-check:
	@test -z "$$(gofmt -l ./cmd ./internal)" || (echo "Run gofmt on:"; gofmt -l ./cmd ./internal; exit 1)

check: fmt-check vet test build

smoke:
	./scripts/smoke.sh

demo-data:
	cp data/items.sample.json data/items.json

refresh:
	@test -n "$$ADMIN_TOKEN" || (echo "Set ADMIN_TOKEN"; exit 1)
	curl --fail --silent --show-error -X POST http://localhost:8080/api/refresh \
		-H "Authorization: Bearer $$ADMIN_TOKEN"

docker-up:
	docker compose up --build

docker-down:
	docker compose down
