package registry

import "testing"

func TestOCIPublisher_BadChartDir(t *testing.T) {
	// We can't easily package a real chart in a unit test, so we verify the
	// error path before the registry client is ever created, same approach
	// as chartmuseum_test.go/ghpages_test.go. This doesn't exercise the
	// PlainHTTP branch specifically (that needs a real or mocked OCI
	// registry to push against) — covered instead by the homelab-artifacts
	// release pipeline actually pushing to Harbor's in-cluster plain-HTTP
	// endpoint.
	p := &OCIPublisher{RegistryURL: "oci://example.invalid/charts"}
	if err := p.Push(t.TempDir(), "0.1.0"); err == nil {
		t.Error("expected error for non-existent chart dir, got nil")
	}
}

func TestOCIPublisher_BadChartDir_PlainHTTP(t *testing.T) {
	// Same as above, with PlainHTTP set — confirms the field doesn't change
	// the (pre-registry-client) error path for a bad chart dir.
	p := &OCIPublisher{RegistryURL: "oci://example.invalid/charts", PlainHTTP: true}
	if err := p.Push(t.TempDir(), "0.1.0"); err == nil {
		t.Error("expected error for non-existent chart dir, got nil")
	}
}
