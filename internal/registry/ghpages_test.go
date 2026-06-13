package registry

import (
	"testing"

	helmchart "helm.sh/helm/v3/pkg/chart"
	helmrepo "helm.sh/helm/v3/pkg/repo"
)

func TestMergeIndex_NewEntryAdded(t *testing.T) {
	dst := helmrepo.NewIndexFile()
	src := helmrepo.NewIndexFile()

	if err := src.MustAdd(&helmchart.Metadata{
		Name:    "myapp",
		Version: "0.2.0",
	}, "myapp-0.2.0.tgz", "https://example.com", ""); err != nil {
		t.Fatalf("MustAdd: %v", err)
	}

	MergeIndex(dst, src)

	entries, ok := dst.Entries["myapp"]
	if !ok || len(entries) == 0 {
		t.Fatal("expected myapp entry in merged index")
	}
	if entries[0].Version != "0.2.0" {
		t.Errorf("expected version 0.2.0, got %s", entries[0].Version)
	}
}

func TestMergeIndex_PreservesExisting(t *testing.T) {
	dst := helmrepo.NewIndexFile()
	if err := dst.MustAdd(&helmchart.Metadata{
		Name:    "myapp",
		Version: "0.1.0",
	}, "myapp-0.1.0.tgz", "https://example.com", ""); err != nil {
		t.Fatalf("MustAdd: %v", err)
	}

	src := helmrepo.NewIndexFile()
	if err := src.MustAdd(&helmchart.Metadata{
		Name:    "myapp",
		Version: "0.2.0",
	}, "myapp-0.2.0.tgz", "https://example.com", ""); err != nil {
		t.Fatalf("MustAdd: %v", err)
	}

	MergeIndex(dst, src)

	entries := dst.Entries["myapp"]
	if len(entries) < 2 {
		t.Errorf("expected 2 entries after merge, got %d", len(entries))
	}
}
