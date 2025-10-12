.PHONY: help build build-go build-rust build-web test test-web test-cover lint fmt run dev dev-web dev-server clean tidy generate

BIN_DIR ?= bin
APP_NAME ?= shellcn
PKG ?= ./...
RUST_MANIFESTS := $(wildcard rust-modules/*/Cargo.toml)
WEB_DIR := web
WEB_DIST := $(WEB_DIR)/dist

help:
	@echo "ShellCN Platform"
	@echo ""
	@echo "Usage:"
	@echo "  make install       Install project dependencies"
	@echo "  make build         Build the entire project (Frontend + Rust FFI + Go binary)"
	@echo "  make build-go      Build the Go backend binary"
	@echo "  make build-rust    Build Rust FFI modules (if present)"
	@echo "  make build-web     Build the frontend"
	@echo "  make test          Run all tests (Frontend + Go)"
	@echo "  make test-web      Run frontend tests"
	@echo "  make test-cover    Run Go unit tests with coverage"
	@echo "  make lint          Run go vet on the codebase"
	@echo "  make fmt           Format Go sources with gofmt"
	@echo "  make tidy          Sync module dependencies"
	@echo "  make run           Start the Go backend"
	@echo "  make dev           Start backend and frontend concurrently"
	@echo "  make dev-server    Start the Go backend"
	@echo "  make dev-web       Start the frontend dev server"
	@echo "  make clean         Remove build artifacts"

install: tidy
	@if [ ! -d "$(WEB_DIR)/node_modules" ]; then \
		echo "Installing frontend dependencies..."; \
		cd $(WEB_DIR) && pnpm install --frozen-lockfile; \
	fi

build: build-web build-rust build-go

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

build-web:
	@echo "Building frontend..."
	@if [ ! -d "$(WEB_DIR)/node_modules" ]; then \
		echo "Installing frontend dependencies..."; \
		cd $(WEB_DIR) && pnpm install --frozen-lockfile; \
	fi
	@cd $(WEB_DIR) && pnpm build
	@echo "✓ Frontend built at $(WEB_DIST)"

test: test-web
	@echo "Running Go tests..."
	@go test ./...

test-web:
	@echo "Running frontend tests..."
	@cd $(WEB_DIR) && pnpm test run

test-cover:
	@echo "Running tests with coverage..."
	@go test -cover ./...

lint:
	@echo "Running go vet..."
	@go vet $(PKG)

fmt-web:
	@echo "Running frontend tests..."
	@cd $(WEB_DIR) && pnpm format

fmt: fmt-web
	@echo "Formatting code..."
	@find . -name '*.go' -not -path './vendor/*' -not -path './.git/*' -print0 | xargs -0 gofmt -w

format: fmt

tidy:
	@echo "Tidying module dependencies..."
	@go mod tidy

run: dev-server

dev:
	@echo "Starting ShellCN full-stack dev environment..."
	@node scripts/dev.mjs

dev-server:
	@echo "Starting Go backend..."
	@go run ./cmd/server

dev-web:
	@echo "Starting frontend dev server..."
	@cd $(WEB_DIR) && pnpm dev

clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BIN_DIR)
	@rm -f coverage.out
	@rm -rf $(WEB_DIST)
	@echo "✓ Build artifacts cleaned"

generate:
	@echo "Running go generate..."
	@go generate $(PKG)
