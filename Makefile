SHELL := bash

# Export variables from .env (gitignored) when it exists. Go reads the
# Anthropic key only from the environment; nothing here prints it.
LOAD_ENV := set -a; if [ -f .env ]; then . ./.env; fi; set +a

SEED ?= 42

.PHONY: dev devserver test test-go test-ui vet fmt-check lint typecheck build check record

## dev: run the Go dev server (127.0.0.1:8080) and next dev together
dev:
	@go build -o bin/devserver ./cmd/devserver
	@$(LOAD_ENV); \
	trap 'kill $$(jobs -p) 2>/dev/null' EXIT; \
	./bin/devserver & \
	npm run dev & \
	wait -n

## devserver: run only the Go dev server
devserver:
	@$(LOAD_ENV); go run ./cmd/devserver

## test: Go tests with the race detector, then the UI unit tests
test: test-go test-ui

test-go:
	go test -race ./...

test-ui:
	npm test

vet:
	go vet ./...

fmt-check:
	@out=$$(gofmt -l api cmd pkg); if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

lint:
	npm run lint

typecheck:
	npm run typecheck

build:
	npm run build

## check: every quality gate (gofmt, vet, race tests, lint, typecheck, next build)
check: fmt-check vet test lint typecheck build

## record: headless agent run, e.g. make record TARGET=target-c SEED=42 [STEPS=30]
record:
	@if [ -z "$(TARGET)" ]; then echo "usage: make record TARGET=target-c SEED=42 [STEPS=30]"; exit 2; fi
	@$(LOAD_ENV); go run ./cmd/record -target $(TARGET) -seed $(SEED) $(if $(STEPS),-steps $(STEPS),)
