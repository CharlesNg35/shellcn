# syntax=docker/dockerfile:1

FROM node:24-bookworm-slim AS web-build
WORKDIR /src/web
RUN corepack enable
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

FROM golang:1.26-bookworm AS go-build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-build /src/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /out/shellcn ./cmd/server
RUN mkdir -p /out/data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=go-build --chown=nonroot:nonroot /out/data /data
COPY --from=go-build /out/shellcn /shellcn
WORKDIR /data
EXPOSE 8081
ENTRYPOINT ["/shellcn"]
