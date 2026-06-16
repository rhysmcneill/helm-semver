# Contributing to helm-semver

Thank you for your interest in contributing to helm-semver! This document covers everything you need to get a development environment running, the standards we hold contributions to, and how to submit a pull request.

---

## Prerequisites

| Tool | Version | Notes |
|------|---------|-------|
| [Go](https://golang.org/dl/) | 1.26+ | See `go.mod` for the exact minimum |
| [GNU Make](https://www.gnu.org/software/make/) | any | Used for all dev tasks |
| [golangci-lint](https://golangci-lint.run/usage/install/) | latest | Installed via `make setup` |
| [Helm CLI](https://helm.sh/docs/intro/install/) | v3 | Required for local chart operations |

---

## Getting Started

```bash
# 1. Clone the repository
git clone https://github.com/rhysmcneill/helm-semver.git
cd helm-semver

# 2. Install pre-commit (one-time, per machine)
pip install pre-commit   # or: brew install pre-commit

# 3. Install development tools (golangci-lint, goimports, pre-commit hooks)
make setup

# 4. Verify the build
make build

# 5. Run the unit test suite
make test

# 6. Run the linter
make lint
```

The compiled binary lands at `bin/helm-semver`.

`make setup` also installs the project's [pre-commit](https://pre-commit.com/)
hooks (configured in `.pre-commit-config.yaml`). Hooks run automatically on
every `git commit` and mirror the local Make targets — formatting, `go vet`,
`golangci-lint`, plus light file-hygiene checks (trailing whitespace, final
newlines, merge-conflict markers, accidental large binaries) and a `gosec`
static-security pass. To run them manually across the whole tree:

```bash
pre-commit run --all-files
```

If `pre-commit` isn't on your `PATH` when `make setup` runs, the make target
prints an install hint and continues — you can re-run `make setup` after
installing it.

---

## Available Make Targets

| Target | Description |
|--------|-------------|
| `make build` | Build the binary for the current platform |
| `make build-all` | Cross-compile for all supported platforms |
| `make install` | Install the binary to `$GOPATH/bin` |
| `make test` | Run all unit tests |
| `make test-cover` | Run unit tests with a coverage report |
| `make lint` | Run golangci-lint |
| `make fmt` | Run `gofmt` and `goimports` across the repo |
| `make vet` | Run `go vet` |
| `make e2e` | Run CLI smoke tests (no external services required) |
| `make docs` | Regenerate CLI flag reference in README.md |
| `make setup` | Install development tools |
| `make ci` | Run everything CI runs (vet + test + build + e2e) |

---

## Code Style

- Follow standard [Go conventions](https://go.dev/doc/effective_go) and the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments) guide.
- All code must pass `go vet` and `golangci-lint run` without warnings.
- Format code with `gofmt` before committing (`make fmt`).
- Keep functions small and focused. Prefer returning errors over panicking.
- Avoid adding comments that just narrate what the code already clearly says.

---

## Testing Standards

### Unit tests

- Unit tests live alongside source files: `internal/semver/semver_test.go` tests `internal/semver/semver.go`.
- Use the standard `testing` package — no third-party assertion libraries.
- Use table-driven tests wherever multiple input/output pairs are being validated. See `internal/semver/semver_test.go` for the pattern.
- Tests must pass with `go test -race ./...`.

### E2E / smoke tests

Smoke tests live in `e2e/` and test the compiled binary against a temporary
in-process git repository. They require no external services and run in CI on
every PR.

```bash
# Build binary first, then run smoke tests
make e2e
```

### Coverage

Aim to keep unit test coverage above 60% for packages in `internal/`. Check coverage with:

```bash
make test-cover
```

---

## Commit Messages

All commit messages must follow the format:

```
<type>(<scope>): <message>
```

Use the imperative mood in `<message>` (e.g., "add support for..." not "added support for..."). Keep the subject line under 72 characters. Separate the subject from the body with a blank line, and use the body to explain *what* and *why*, not *how*.

> [!IMPORTANT]
> Commit prefixes directly drive the version bump logic in helm-semver itself.
> `fix:` → patch, `feat:` → minor, `feat!:` → major. Use them correctly.

### Types

| Type | Purpose | Version bump |
|------|---------|-------------|
| `feat` | New feature or user-facing functionality | minor |
| `fix` | Bug fix | patch |
| `feat!` | Breaking change | major |
| `refactor` | Code change that neither fixes a bug nor adds a feature | none |
| `test` | Adding or updating tests | none |
| `chore` | Maintenance, tooling, CI, dependencies | none |
| `perf` | Performance improvement | none |
| `docs` | Documentation only | none |
| `ci` | CI/CD configuration changes | none |

---

## Pull Request Checklist

Before opening a PR, verify:

- [ ] `make ci` passes locally
- [ ] New behaviour is covered by tests
- [ ] No linter warnings (`make lint`)
- [ ] PR title follows the commit message convention above (validated automatically by CI)

---

## Project Layout

```text
helm-semver/
├── cmd/helm-semver/     # binary entry point — keep thin
├── e2e/                 # smoke tests against the compiled binary
├── internal/
│   ├── changelog/       # CHANGELOG.md generation
│   ├── chart/           # Chart.yaml read/write
│   ├── git/             # git log, commit, tag, push (go-git)
│   ├── registry/        # OCI, ChartMuseum, GitHub Pages publishers
│   ├── release/         # GitHub Releases API
│   ├── semver/          # conventional commit parsing + version bumping
│   └── version/         # build-time version variables (set via ldflags)
├── docs/                # registry-specific auth guides
├── Makefile
├── go.mod
└── go.sum
```
