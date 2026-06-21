BINARY := kkachi-agent-helper
BIN_DIR := bin
VERSION ?= 0.1.13
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
DIST_DIR ?= dist
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
TOOLCHAIN_ROOT ?= $(HOME)/.local/kkachi/toolchains
TOOLCHAIN_COMPONENT := kah
TOOLCHAIN_VERSION ?= $(shell git describe --tags --exact-match 2>/dev/null | sed 's/^v//' || true)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)

.PHONY: build install install-toolchain release format vet lint test-prepare test-unit test-int test-e2e test check clean

build:
	mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) .

install:
	env -u GOPATH go install -ldflags "$(LDFLAGS)" .

install-toolchain:
	@set -e; \
	VERSION_VALUE="$(VERSION)"; \
	if [ "$$VERSION_VALUE" = "0.0.0-dev" ]; then VERSION_VALUE="$(TOOLCHAIN_VERSION)"; fi; \
	VERSION_VALUE="$${VERSION_VALUE#v}"; \
	if [ -z "$$VERSION_VALUE" ] || [ "$$VERSION_VALUE" = "0.0.0-dev" ]; then \
		echo "ERROR: install-toolchain requires a real version; set VERSION=0.1.x or run from an exact v0.1.x tag" >&2; \
		exit 1; \
	fi; \
	$(MAKE) build VERSION="$$VERSION_VALUE"; \
	VERSION_TAG="v$$VERSION_VALUE"; \
	case "$$VERSION_TAG" in v[0-9]*.[0-9]*.[0-9]*) ;; *) echo "ERROR: unsupported $(BINARY) version for toolchain install: $$VERSION_VALUE" >&2; exit 1 ;; esac; \
	INSTALL_DIR="$(TOOLCHAIN_ROOT)/$(TOOLCHAIN_COMPONENT)/$$VERSION_TAG/bin"; \
	mkdir -p "$$INSTALL_DIR"; \
	install -m 0755 "$(BIN_DIR)/$(BINARY)" "$$INSTALL_DIR/$(BINARY)"; \
	INSTALLED_VERSION="$$("$$INSTALL_DIR/$(BINARY)" --version | awk '{print $$2}')"; \
	if [ "$${INSTALLED_VERSION#v}" != "$$VERSION_VALUE" ]; then \
		echo "ERROR: installed version mismatch: expected $$VERSION_VALUE, got $$INSTALLED_VERSION" >&2; \
		exit 1; \
	fi; \
	echo "installed $(BINARY) $$VERSION_TAG to $$INSTALL_DIR/$(BINARY)"

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
