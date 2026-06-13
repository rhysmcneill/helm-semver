// Package e2e runs smoke tests against the compiled helm-semver binary using
// a temporary git repository. No external services are required.
package e2e

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// binaryPath returns the path to the compiled binary.
func binaryPath() string {
	bin := "helm-semver"
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	// Makefile puts the binary at bin/helm-semver from repo root.
	root, _ := repoRoot()
	return filepath.Join(root, "bin", bin)
}

func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		dir = parent
	}
}

func run(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	cmd := exec.Command(binaryPath(), args...) //nolint:gosec // binary path is resolved internally
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func TestBinary_Version(t *testing.T) {
	if _, err := os.Stat(binaryPath()); os.IsNotExist(err) {
		t.Skip("binary not built — run 'make build' first")
	}

	out, _, err := run(t, "version")
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if !strings.Contains(out, "helm-semver") {
		t.Errorf("version output = %q, want to contain 'helm-semver'", out)
	}
}

func TestBinary_DryRun(t *testing.T) {
	if _, err := os.Stat(binaryPath()); os.IsNotExist(err) {
		t.Skip("binary not built — run 'make build' first")
	}

	// Set up a temp git repo with a chart.
	dir := t.TempDir()
	repo, err := gogit.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("init repo: %v", err)
	}
	wt, _ := repo.Worktree()

	// Create a minimal chart.
	chartDir := filepath.Join(dir, "charts", "myapp")
	_ = os.MkdirAll(chartDir, 0o750)
	_ = os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte(`apiVersion: v2
name: myapp
version: 0.1.0
`), 0o600)

	_, _ = wt.Add("charts/myapp/Chart.yaml")
	_, err = wt.Commit("feat: initial chart", &gogit.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@test.com"},
	})
	if err != nil {
		t.Fatalf("initial commit: %v", err)
	}

	// Run release in dry-run mode.
	cmd := exec.Command(binaryPath(), //nolint:gosec // args are test-internal constants
		"release",
		"--registry", "oci://ghcr.io/test-org/helm-charts",
		"--registry-type", "oci",
		"--charts-dir", "charts",
		"--dry-run",
		"--changelog=false",
	)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()

	out := stdout.String() + stderr.String()
	if err != nil {
		t.Logf("stdout: %s", stdout.String())
		t.Logf("stderr: %s", stderr.String())
		t.Fatalf("dry-run failed: %v", err)
	}

	if !strings.Contains(out, "myapp") {
		t.Errorf("expected 'myapp' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "dry-run") {
		t.Errorf("expected '[dry-run]' marker in output, got:\n%s", out)
	}
}
