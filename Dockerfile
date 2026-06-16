FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 go build \
  -ldflags "-s -w \
    -X github.com/rmcneill/helm-semver/internal/version.Version=${VERSION} \
    -X github.com/rmcneill/helm-semver/internal/version.Commit=${COMMIT} \
    -X github.com/rmcneill/helm-semver/internal/version.BuildDate=${BUILD_DATE}" \
  -o /helm-semver ./cmd/helm-semver

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /helm-semver /helm-semver
ENTRYPOINT ["/helm-semver"]
