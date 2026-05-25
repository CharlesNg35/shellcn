.PHONY: help install build build-go build-web test test-web test-cover lint lint-go lint-web fmt fmt-go fmt-web tidy run dev dev-web dev-api dev-server clean tools

BIN_DIR ?= bin
APP_NAME ?= shellcn
PKG ?= ./...
WEB_DIR := web
WEB_DIST := $(WEB_DIR)/dist
GO_LDFLAGS ?= -s -w

help:
	@echo "ShellCN — make targets"
	@echo ""
	@echo "  build       Build frontend + embed + Go binary ($(BIN_DIR)/$(APP_NAME))"
	@echo "  build-web   Build the frontend (web/dist)"
	@echo "  build-go    Build the Go binary (embeds web/dist)"
	@echo "  test        Run frontend + Go tests"
	@echo "  lint        Lint frontend + Go"
	@echo "  fmt         Format frontend + Go"
	@echo "  dev         Run API + web dev servers concurrently"
	@echo "  dev-api     Run the Go API (--dev) with live reload"
	@echo "  dev-web     Run the Vite dev server"
	@echo "  install     Install Go + frontend dependencies"
	@echo "  tools       Install dev tools (wgo, gofumpt)"
	@echo "  clean       Remove build artifacts"

install:
	@go mod download
	@cd $(WEB_DIR) && pnpm install --frozen-lockfile

build: build-web build-go

build-web:
	@echo "Building frontend..."
	@if [ ! -d "$(WEB_DIR)/node_modules" ]; then cd $(WEB_DIR) && pnpm install --frozen-lockfile; fi
	@cd $(WEB_DIR) && pnpm build
	@echo "✓ Frontend built at $(WEB_DIST)"

build-go:
	@echo "Building Go binary..."
	@mkdir -p $(BIN_DIR)
	@CGO_ENABLED=0 go build -ldflags "$(GO_LDFLAGS)" -o $(BIN_DIR)/$(APP_NAME) ./cmd/server
	@echo "✓ Binary at $(BIN_DIR)/$(APP_NAME)"

test: test-web
	@echo "Running Go tests..."
	@go test -race $(PKG)

test-web:
	@echo "Running frontend unit tests..."
	@cd $(WEB_DIR) && pnpm test:unit

test-cover:
	@go test -coverprofile=coverage.out $(PKG)

lint: lint-web lint-go

lint-go:
	@echo "Linting Go..."
	@if command -v golangci-lint >/dev/null 2>&1; then golangci-lint run $(PKG); else echo "golangci-lint not found; using go vet"; go vet $(PKG); fi

lint-web:
	@echo "Linting frontend..."
	@cd $(WEB_DIR) && pnpm lint

fmt: fmt-web fmt-go

fmt-go:
	@if command -v golangci-lint >/dev/null 2>&1; then golangci-lint fmt $(PKG); else gofmt -w cmd internal plugins; fi

fmt-web:
	@cd $(WEB_DIR) && pnpm format

tidy:
	@go mod tidy

run: build-go
	@$(BIN_DIR)/$(APP_NAME)

dev:
	@node scripts/dev.mjs

dev-api:
	@if command -v wgo >/dev/null 2>&1; then wgo run ./cmd/server --dev; else echo "wgo not found (run 'make tools'); running without live reload"; go run ./cmd/server --dev; fi

dev-server: dev-api

dev-web:
	@cd $(WEB_DIR) && pnpm dev

clean:
	@rm -rf $(BIN_DIR) coverage.out $(WEB_DIST) tmp
	@echo "✓ cleaned"

tools:
	@go install github.com/bokwoon95/wgo@latest
	@go install mvdan.cc/gofumpt@latest
