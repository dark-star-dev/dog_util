.PHONY: all build test run fmt vet lint install clean release release-linux-amd64 release-linux-arm64 release-darwin-amd64 release-darwin-arm64 help

BINARY := dog
GO ?= go
GOLANGCI_LINT ?= golangci-lint
DIST_DIR ?= dist

UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)

ifeq ($(UNAME_S),Darwin)
  ifeq ($(UNAME_M),arm64)
    DEFAULT_PREFIX := /opt/homebrew
  else
    DEFAULT_PREFIX := /usr/local
  endif
else
  DEFAULT_PREFIX := /usr/local
endif

PREFIX ?= $(DEFAULT_PREFIX)
BINDIR ?= $(PREFIX)/bin

all: build

help:
	@printf "Usage:\n"
	@printf "  make build        Build the local binary\n"
	@printf "  make test         Run the Go test suite\n"
	@printf "  make fmt          Format Go sources\n"
	@printf "  make vet          Run go vet\n"
	@printf "  make lint         Run golangci-lint (if installed)\n"
	@printf "  make install      Install to $(BINDIR)\n"
	@printf "  make release      Build release archives for Linux/macOS\n"
	@printf "  make clean        Remove build artifacts\n"

build:
	$(GO) build -o $(BINARY) .

test:
	$(GO) test ./...

run:
	$(GO) run .

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

lint:
	@if ! command -v $(GOLANGCI_LINT) >/dev/null 2>&1; then \
		echo "golangci-lint not found. Install it with: "; \
		echo "  brew install golangci-lint"; \
		echo "  or"; \
		echo "  $(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		exit 1; \
	fi
	$(GOLANGCI_LINT) run ./...

install:
	install -d $(BINDIR)
	install -m 0755 $(BINARY) $(BINDIR)/$(BINARY)

clean:
	rm -rf $(DIST_DIR) $(BINARY)

release: release-dir release-linux-amd64 release-linux-arm64 release-darwin-amd64 release-darwin-arm64

release-dir:
	@mkdir -p $(DIST_DIR)

release-linux-amd64:
	GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags="-s -w" -o $(DIST_DIR)/$(BINARY)-linux-amd64 .

release-linux-arm64:
	GOOS=linux GOARCH=arm64 $(GO) build -trimpath -ldflags="-s -w" -o $(DIST_DIR)/$(BINARY)-linux-arm64 .

release-darwin-amd64:
	GOOS=darwin GOARCH=amd64 $(GO) build -trimpath -ldflags="-s -w" -o $(DIST_DIR)/$(BINARY)-darwin-amd64 .

release-darwin-arm64:
	GOOS=darwin GOARCH=arm64 $(GO) build -trimpath -ldflags="-s -w" -o $(DIST_DIR)/$(BINARY)-darwin-arm64 .
