.PHONY: help build test lint fmt run dev clean tidy generate

BIN_DIR ?= bin
APP_NAME ?= shellcn
PKG ?= ./...

help:
	@echo "ShellCN Backend"
	@echo ""
	@echo "Usage:"
	@echo "  make build     Build the backend binary to $(BIN_DIR)/$(APP_NAME)"
	@echo "  make test      Run Go unit tests"
	@echo "  make lint      Run go vet on the codebase"
	@echo "  make fmt       Format Go sources with gofmt"
	@echo "  make tidy      Sync module dependencies"
	@echo "  make run       Start the development server"
	@echo "  make clean     Remove build artifacts"

build:
	@echo "Building backend..."
	@mkdir -p $(BIN_DIR)
	@CGO_ENABLED=1 go build -o $(BIN_DIR)/$(APP_NAME) ./cmd/server
	@echo "✓ Binary created at $(BIN_DIR)/$(APP_NAME)"

test:
	@echo "Running tests..."
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
