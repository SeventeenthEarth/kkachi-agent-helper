BINARY := kkachi-agent-helper
BIN_DIR := bin
VERSION ?= 0.0.0-dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
PREFIX ?= $(HOME)/.local
DIST_DIR ?= dist
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)

.PHONY: build install-local release format vet lint test-prepare test-unit test-int test-e2e test check clean

build:
	mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) .

install-local: build
	mkdir -p $(PREFIX)/bin
	install -m 0755 $(BIN_DIR)/$(BINARY) $(PREFIX)/bin/$(BINARY)

release:
	VERSION=$(VERSION) COMMIT=$(COMMIT) BUILD_DATE=$(BUILD_DATE) DIST_DIR=$(DIST_DIR) GOOS=$(GOOS) GOARCH=$(GOARCH) ./scripts/build-release.sh

format:
	gofmt -w main.go ./cmd ./internal ./tests

vet:
	go vet ./...
	go vet -tags=integration ./tests/integration

lint: vet

test-prepare: format vet lint

test-unit:
	go test . ./cmd/... ./internal/...

test-int:
	go test -tags=integration ./tests/integration

test-e2e:
	./scripts/test-e2e.sh

test: test-prepare test-unit test-int test-e2e

check: test build

clean:
	rm -rf $(BIN_DIR) $(DIST_DIR)
