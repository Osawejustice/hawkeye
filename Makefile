APP        := cohi-api
WORKER     := cohi-worker
MODULE     := github.com/cohi-hq/cohi-api
GO         ?= go
BIN        := bin/$(APP)
WORKER_BIN := bin/$(WORKER)

.PHONY: help run run-worker build test vet tidy migrate-up migrate-down docker-up docker-down docker-build air

help:
	@echo "Targets:"
	@echo "  run            go run ./cmd/api (requires Postgres)"
	@echo "  run-worker     go run ./cmd/worker"
	@echo "  build          compile api + worker to bin/"
	@echo "  test           go test ./..."
	@echo "  vet            go vet ./..."
	@echo "  tidy           go mod tidy"
	@echo "  docker-up      start Postgres + MediaMTX + API"
	@echo "  docker-down    stop the local stack"
	@echo "  docker-build   rebuild the API image"
	@echo "  air            hot reload via air (go install github.com/air-verse/air@latest)"

run:
	$(GO) run ./cmd/api

run-worker:
	$(GO) run ./cmd/worker

build:
	mkdir -p bin
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w" -o $(BIN) ./cmd/api
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w" -o $(WORKER_BIN) ./cmd/worker

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

tidy:
	$(GO) mod tidy

docker-up:
	docker compose up -d --build

docker-down:
	docker compose down

docker-build:
	docker compose build api

air:
	air
