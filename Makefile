.PHONY: help build build-rust build-frontend build-backend test test-backend test-frontend clean run dev install-deps lint fmt check

# Default target
help:
	@echo "ShellCN Platform - Makefile Commands"
	@echo ""
	@echo "Build Commands:"
	@echo "  make build              - Build complete application (Rust + Frontend + Backend)"
	@echo "  make build-rust         - Build Rust FFI modules (RDP, VNC)"
	@echo "  make build-frontend     - Build React frontend"
	@echo "  make build-backend      - Build Go backend binary"
	@echo ""
	@echo "Development Commands:"
	@echo "  make dev                - Run development server (hot reload)"
	@echo "  make run                - Run the application"
	@echo "  make install-deps       - Install all dependencies"
	@echo ""
	@echo "Testing Commands:"
	@echo "  make test               - Run all tests (backend + frontend)"
	@echo "  make test-backend       - Run Go tests"
	@echo "  make test-frontend      - Run React tests"
	@echo ""
	@echo "Code Quality:"
	@echo "  make lint               - Run linters (Go + TypeScript)"
	@echo "  make fmt                - Format code (Go + TypeScript)"
	@echo "  make check              - Run all checks (lint + test)"
	@echo ""
	@echo "Cleanup:"
	@echo "  make clean              - Clean build artifacts"
	@echo ""

# Complete build process
build: build-rust build-frontend build-backend
	@echo "✓ Build complete! Binary available at: ./shellcn"

# Build Rust FFI modules (RDP, VNC)
build-rust:
	@echo "Building Rust FFI modules..."
	@cd rust-modules/rdp && cargo build --release
	@cd rust-modules/vnc && cargo build --release
	@echo "✓ Rust FFI modules built"

# Build React frontend
build-frontend:
	@echo "Building frontend..."
	@cd web && pnpm install && pnpm run build
	@echo "✓ Frontend built to web/dist/"

# Build Go backend (with embedded frontend)
build-backend:
	@echo "Building Go backend..."
	@CGO_ENABLED=1 go build -o shellcn ./cmd/server
	@echo "✓ Backend binary created: ./shellcn"

# Install dependencies
install-deps:
	@echo "Installing dependencies..."
	@echo "Installing Go dependencies..."
	@go mod download
	@echo "Installing frontend dependencies..."
	@cd web && pnpm install
	@echo "Installing Rust toolchain (if needed)..."
	@rustc --version || curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
	@echo "✓ Dependencies installed"

# Run tests
test: test-backend test-frontend
	@echo "✓ All tests passed"

# Run Go tests
test-backend:
	@echo "Running Go tests..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out
	@echo "✓ Backend tests complete"

# Run frontend tests
test-frontend:
	@echo "Running frontend tests..."
	@cd web && pnpm test
	@echo "✓ Frontend tests complete"

# Run the application
run:
	@echo "Starting ShellCN Platform..."
	@./shellcn

# Development mode (with hot reload)
dev:
	@echo "Starting development server..."
	@echo "Backend will run on http://localhost:8080"
	@echo "Frontend dev server on http://localhost:5173"
	@echo ""
	@trap 'kill 0' EXIT; \
		(cd web && pnpm run dev) & \
		air || go run ./cmd/server

# Linting
lint:
	@echo "Running linters..."
	@echo "Linting Go code..."
	@golangci-lint run ./... || echo "Install golangci-lint: https://golangci-lint.run/usage/install/"
	@echo "Linting frontend code..."
	@cd web && pnpm run lint
	@echo "✓ Linting complete"

# Format code
fmt:
	@echo "Formatting code..."
	@echo "Formatting Go code..."
	@gofmt -w .
	@go mod tidy
	@echo "Formatting frontend code..."
	@cd web && pnpm run format
	@echo "✓ Code formatted"

# Run all checks
check: lint test
	@echo "✓ All checks passed"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -f shellcn
	@rm -f coverage.out
	@rm -rf web/dist
	@cd rust-modules/rdp && cargo clean
	@cd rust-modules/vnc && cargo clean
	@echo "✓ Clean complete"

# Database migrations (when using external DB)
migrate-up:
	@echo "Running database migrations..."
	@go run ./cmd/migrate up
	@echo "✓ Migrations applied"

migrate-down:
	@echo "Rolling back database migrations..."
	@go run ./cmd/migrate down
	@echo "✓ Migrations rolled back"

# Docker build (optional)
docker-build:
	@echo "Building Docker image..."
	@docker build -t shellcn:latest .
	@echo "✓ Docker image built"

docker-run:
	@echo "Running Docker container..."
	@docker run -p 8080:8080 -v $(PWD)/data:/app/data shellcn:latest

# Generate mocks for testing
generate-mocks:
	@echo "Generating mocks..."
	@go generate ./...
	@echo "✓ Mocks generated"

# Check for outdated dependencies
check-deps:
	@echo "Checking for outdated dependencies..."
	@echo "Go dependencies:"
	@go list -u -m all
	@echo ""
	@echo "Frontend dependencies:"
	@cd web && pnpm outdated
	@echo ""
	@echo "Rust dependencies:"
	@cd rust-modules/rdp && cargo outdated || echo "Install cargo-outdated: cargo install cargo-outdated"
	@cd rust-modules/vnc && cargo outdated || echo "Install cargo-outdated: cargo install cargo-outdated"
