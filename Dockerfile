FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build \
  -ldflags "-s -w \
    -X github.com/rmcneill/helm-semver/internal/version.Version=$(git describe --tags --always --dirty 2>/dev/null || echo dev) \
    -X github.com/rmcneill/helm-semver/internal/version.Commit=$(git rev-parse --short HEAD 2>/dev/null || echo none) \
    -X github.com/rmcneill/helm-semver/internal/version.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -o /helm-semver ./cmd/helm-semver

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /helm-semver /helm-semver
ENTRYPOINT ["/helm-semver"]
