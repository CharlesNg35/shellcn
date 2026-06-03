# syntax=docker/dockerfile:1

FROM golang:1.26.4-bookworm AS go-build
WORKDIR /src
# The root module replaces github.com/charlesng35/shellcn/sdk with the nested
# ./sdk module, so its go.mod must be present before `go mod download`.
COPY go.mod go.sum ./
COPY sdk/go.mod sdk/go.sum ./sdk/
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /out/shellcn-agent ./cmd/agent

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=go-build /out/shellcn-agent /shellcn-agent
ENTRYPOINT ["/shellcn-agent"]
