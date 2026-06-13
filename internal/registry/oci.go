package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"helm.sh/helm/v3/pkg/action"
	helmchart "helm.sh/helm/v3/pkg/chart/loader"
	helmregistry "helm.sh/helm/v3/pkg/registry"
)

// OCIPublisher pushes charts to an OCI-compliant registry (GHCR, ECR, ACR…).
type OCIPublisher struct {
	// RegistryURL is the OCI registry URL, e.g. "oci://ghcr.io/my-org/helm-charts".
	RegistryURL string
	// Username and Password for registry authentication.
	Username string
	Password string
}

// Push packages the chart at chartDir and pushes it to the OCI registry.
func (p *OCIPublisher) Push(chartDir, version string) error {
	// Load chart metadata to extract the name.
	ch, err := helmchart.Load(chartDir)
	if err != nil {
		return fmt.Errorf("loading chart at %s: %w", chartDir, err)
	}

	// Create a temp dir for the packaged .tgz.
	tmpDir, err := os.MkdirTemp("", "helm-semver-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir) //nolint:errcheck

	// Package the chart.
	pkg := action.NewPackage()
	pkg.Destination = tmpDir
	tgzPath, err := pkg.Run(chartDir, nil)
	if err != nil {
		return fmt.Errorf("packaging chart %s: %w", ch.Name(), err)
	}

	// Create a registry client.
	var clientOpts []helmregistry.ClientOption
	if p.Username != "" && p.Password != "" {
		clientOpts = append(clientOpts,
			helmregistry.ClientOptBasicAuth(p.Username, p.Password),
		)
	}
	client, err := helmregistry.NewClient(clientOpts...)
	if err != nil {
		return fmt.Errorf("creating registry client: %w", err)
	}

	// Read the packaged chart.
	data, err := os.ReadFile(tgzPath) //nolint:gosec
	if err != nil {
		return fmt.Errorf("reading packaged chart: %w", err)
	}

	// Push to the OCI registry.
	// Strip the "oci://" scheme prefix that helm registry login requires.
	registryBase := strings.TrimPrefix(p.RegistryURL, helmregistry.OCIScheme+"://")
	ref := fmt.Sprintf("%s/%s:%s",
		registryBase,
		filepath.Base(ch.Name()),
		version,
	)
	_, err = client.Push(data, ref)
	if err != nil {
		return fmt.Errorf("pushing %s to %s: %w", ch.Name(), p.RegistryURL, err)
	}

	return nil
}
