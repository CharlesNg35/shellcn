# syntax=docker/dockerfile:1

FROM golang:1.25-bookworm AS go-build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /out/shellcn-agent ./cmd/agent

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=go-build /out/shellcn-agent /shellcn-agent
ENTRYPOINT ["/shellcn-agent"]
