package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionCmd(t *testing.T) {
	root := newRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"version"})

	if err := root.Execute(); err != nil {
		t.Fatalf("version cmd error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "helm-semver") {
		t.Errorf("version output missing 'helm-semver': %q", out)
	}
}

func TestReleaseCmd_RequiresRegistry(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{"release"})
	// Should fail because --registry is required.
	err := root.Execute()
	if err == nil {
		t.Error("expected error when --registry is missing, got nil")
	}
}

func TestReleaseCmd_UnknownRegistryType(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{
		"release",
		"--registry", "https://example.com",
		"--registry-type", "unknown",
		"--dry-run",
	})
	err := root.Execute()
	if err == nil {
		t.Error("expected error for unknown registry type, got nil")
	}
}
