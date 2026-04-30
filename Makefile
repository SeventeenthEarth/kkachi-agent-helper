BINARY := kkachi-agent-helper
BIN_DIR := bin
VERSION ?= 0.0.0-dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)

.PHONY: build format vet lint test-prepare test-unit test-int test-e2e test check clean

build:
	mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) ./cmd/kkachi-agent-helper

format:
	gofmt -w ./cmd ./internal ./tests

vet:
	go vet ./...
	go vet -tags=integration ./tests/integration

lint: vet

test-prepare: format vet lint

test-unit:
	go test ./cmd/... ./internal/...

test-int:
	go test -tags=integration ./tests/integration

test-e2e:
	./scripts/test-e2e.sh

test: test-prepare test-unit test-int test-e2e

check: test build

clean:
	rm -rf $(BIN_DIR)
