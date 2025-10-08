.PHONY: help build build-go build-rust test test-cover lint fmt run dev clean tidy generate

BIN_DIR ?= bin
APP_NAME ?= shellcn
PKG ?= ./...
RUST_MANIFESTS := $(wildcard rust-modules/*/Cargo.toml)

help:
	@echo "ShellCN Backend"
	@echo ""
	@echo "Usage:"
	@echo "  make build         Build the backend (Rust FFI + Go binary)"
	@echo "  make build-go      Build the Go backend binary"
	@echo "  make build-rust    Build Rust FFI modules (if present)"
	@echo "  make test          Run Go unit tests"
	@echo "  make test-cover    Run Go unit tests with coverage"
	@echo "  make lint          Run go vet on the codebase"
	@echo "  make fmt           Format Go sources with gofmt"
	@echo "  make tidy          Sync module dependencies"
	@echo "  make run           Start the development server"
	@echo "  make clean         Remove build artifacts"

build: build-rust build-go

build-go:
	@echo "Building Go backend..."
	@mkdir -p $(BIN_DIR)
	@CGO_ENABLED=1 go build -o $(BIN_DIR)/$(APP_NAME) ./cmd/server
	@echo "✓ Go binary created at $(BIN_DIR)/$(APP_NAME)"

build-rust:
ifneq ($(strip $(RUST_MANIFESTS)),)
	@echo "Building Rust FFI modules..."
	@set -e; \
	for manifest in $(RUST_MANIFESTS); do \
		dir=$$(dirname $$manifest); \
		echo "  • $$dir"; \
		if [ -f "$$dir/Cargo.lock" ]; then \
			( cd "$$dir" && cargo build --locked --release ); \
		else \
			( cd "$$dir" && cargo build --release ); \
		fi \
	done
	@echo "✓ Rust FFI modules built"
else
	@echo "No Rust modules detected; skipping Rust build."
endif

test:
	@echo "Running tests..."
	@go test ./...

test-cover:
	@echo "Running tests with coverage..."
	@go test -cover ./...

lint:
	@echo "Running go vet..."
	@go vet $(PKG)

fmt:
	@echo "Formatting code..."
	@find . -name '*.go' -not -path './vendor/*' -not -path './.git/*' -print0 | xargs -0 gofmt -w

tidy:
	@echo "Tidying module dependencies..."
	@go mod tidy

run:
	@echo "Starting development server..."
	@go run ./cmd/server

dev: run

clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BIN_DIR)
	@rm -f coverage.out

generate:
	@echo "Running go generate..."
	@go generate $(PKG)
