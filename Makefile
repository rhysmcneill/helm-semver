# NOTE: This Makefile uses POSIX shell utilities (git, date, etc.) and is
# intended to be run on Linux or macOS. Windows developers must use WSL,
# Git Bash, or MSYS2 to invoke make targets locally.

VERSION  ?= $(shell git describe --tags --always --dirty)
COMMIT   ?= $(shell git rev-parse --short HEAD)
DATE     ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS  = -ldflags "\
	-X github.com/rmcneill/helm-semver/internal/version.Version=$(VERSION) \
	-X github.com/rmcneill/helm-semver/internal/version.Commit=$(COMMIT) \
	-X github.com/rmcneill/helm-semver/internal/version.BuildDate=$(DATE)"

BINARY   = bin/helm-semver

.PHONY: build build-all test test-cover lint fmt vet install setup docs e2e ci pre-commit-hooks-update

# ── Build ──────────────────────────────────────────────────────────────────────

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/helm-semver

build-all:
	GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o bin/helm-semver-linux-amd64       ./cmd/helm-semver
	GOOS=linux   GOARCH=arm64 go build $(LDFLAGS) -o bin/helm-semver-linux-arm64       ./cmd/helm-semver
	GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -o bin/helm-semver-darwin-amd64      ./cmd/helm-semver
	GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o bin/helm-semver-darwin-arm64      ./cmd/helm-semver
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/helm-semver-windows-amd64.exe ./cmd/helm-semver

install:
	go install $(LDFLAGS) ./cmd/helm-semver

# ── Test ───────────────────────────────────────────────────────────────────────

test:
	go test ./...

test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

# Run CLI smoke tests against the compiled binary (no external services required).
e2e: build
	go test ./e2e/ -v -count=1

# ── Code quality ───────────────────────────────────────────────────────────────

fmt:
	gofmt -w -s .
	goimports -w .

vet:
	go vet ./...

lint:
	golangci-lint run

# ── Docs ───────────────────────────────────────────────────────────────────────

# Regenerate the CLI flag reference embedded in README.md.
docs: build
	go run ./cmd/helm-semver docs --output README.md

# ── Developer setup ────────────────────────────────────────────────────────────

# Install development tooling. Run once after cloning.
setup:
	GOTOOLCHAIN=local go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	@if command -v pre-commit >/dev/null 2>&1; then \
		pre-commit install; \
		pre-commit install --hook-type commit-msg; \
	else \
		echo "pre-commit not found — install via 'pip install pre-commit' or 'brew install pre-commit', then re-run 'make setup'"; \
	fi

# ── Pre-commit cleanup ─────────────────────────────────────────────────────────

pre-commit-hooks-update:
	pre-commit clean
	pre-commit install-hooks

# ── CI ─────────────────────────────────────────────────────────────────────────

# Full local CI check — mirrors what the CI workflow runs on PRs.
ci: vet test build e2e
