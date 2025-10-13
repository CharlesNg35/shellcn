ARG GO_VERSION=1.24
ARG RUST_VERSION=1.75
ARG NODE_VERSION=20

# ============================================================================
# Stage 1: Frontend Builder
# ============================================================================
FROM node:${NODE_VERSION}-bookworm-slim AS frontend-builder

WORKDIR /src

# Install pnpm
RUN npm install pnpm@latest -g

# Copy frontend package files
COPY web/package.json web/pnpm-lock.yaml ./web/

# Install dependencies
WORKDIR /src/web
RUN pnpm install --frozen-lockfile

# Copy frontend source
COPY web/ ./

# Build frontend
RUN pnpm build

# ============================================================================
# Stage 2: Rust FFI Builder (Optional - only when FFI modules exist)
# ============================================================================
FROM rust:${RUST_VERSION}-bookworm AS rust-builder

WORKDIR /src

# Create placeholder directory structure
RUN mkdir -p rust-modules lib

# Copy Rust FFI modules if they exist (will copy empty dir if not present)
COPY rust-modules/ ./rust-modules/

# Build all Rust FFI modules dynamically
# cbindgen is installed as build-dependency and runs automatically via build.rs
RUN set -e; \
  for module_dir in rust-modules/*/; do \
    if [ -f "${module_dir}Cargo.toml" ]; then \
      module_name=$(basename "$module_dir"); \
      echo "🔨 Building ${module_name} FFI module..."; \
      cd "$module_dir" && \
      cargo build --release && \
      cd ../.. && \
      lib_file=$(find "${module_dir}target/release" -maxdepth 1 -name "lib*.a" -type f | head -n 1); \
      if [ -n "$lib_file" ]; then \
        cp "$lib_file" lib/; \
        echo "✅ Copied $(basename "$lib_file") to lib/"; \
      fi; \
    fi; \
  done

# ============================================================================
# Stage 3: Go Builder
# ============================================================================
FROM golang:${GO_VERSION}-bookworm AS go-builder

ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG VERSION=dev

WORKDIR /src

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
  build-essential \
  ca-certificates \
  && rm -rf /var/lib/apt/lists/*

# Copy frontend build artifacts
COPY --from=frontend-builder /src/web/dist/ ./web/dist/

# Copy Rust FFI artifacts if they were built
# This includes both the static libraries (.a files) and generated C headers
COPY --from=rust-builder /src/lib/ ./lib/
COPY --from=rust-builder /src/rust-modules/ ./rust-modules/

# Download Go dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the single binary
# CGO is enabled to support SQLite and optional FFI modules
# FFI modules will be statically linked if they exist
RUN set -ex; \
  BUILD_TAGS=""; \
  # Check if any FFI libraries were built
  if [ -n "$(find lib -name 'lib*.a' -type f 2>/dev/null)" ]; then \
    BUILD_TAGS="ffi"; \
    echo "✅ Building with FFI support (tags: ffi)"; \
    echo "📦 FFI libraries found:"; \
    ls -lh lib/*.a 2>/dev/null || true; \
  else \
    echo "ℹ️  Building without FFI support"; \
  fi; \
  CGO_ENABLED=1 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
  go build -trimpath \
  -ldflags="-s -w -X main.Version=${VERSION}" \
  ${BUILD_TAGS:+-tags=$BUILD_TAGS} \
  -o /out/shellcn \
  ./cmd/server

# ============================================================================
# Stage 3: Runtime Image
# ============================================================================
FROM debian:bookworm-slim

ARG VERSION=dev

LABEL org.opencontainers.image.source="https://github.com/charlesng35/shellcn" \
  org.opencontainers.image.description="ShellCN - Enterprise Remote Access Platform" \
  org.opencontainers.image.version="${VERSION}" \
  org.opencontainers.image.licenses="Apache-2.0"

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
  ca-certificates \
  && rm -rf /var/lib/apt/lists/* \
  && useradd --system --create-home --home-dir /var/lib/shellcn --shell /usr/sbin/nologin shellcn

# Environment variables
ENV SHELLCN_DATABASE_PATH=/var/lib/shellcn/data.sqlite \
  SHELLCN_SERVER_PORT=8000

WORKDIR /var/lib/shellcn
VOLUME ["/var/lib/shellcn"]

# Copy the single binary
COPY --from=go-builder /out/shellcn /usr/local/bin/shellcn

USER shellcn
EXPOSE 8000

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/usr/local/bin/shellcn", "health"] || exit 1

ENTRYPOINT ["/usr/local/bin/shellcn"]
