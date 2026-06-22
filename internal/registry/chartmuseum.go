package registry

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"helm.sh/helm/v3/pkg/action"
	helmchart "helm.sh/helm/v3/pkg/chart/loader"
)

// ChartMuseumPublisher pushes charts to a ChartMuseum or Harbor instance.
type ChartMuseumPublisher struct {
	// BaseURL is the ChartMuseum URL, e.g. "https://charts.my-org.com".
	BaseURL string
	// Username and Password for basic auth (optional).
	Username string
	Password string
}

// Push packages the chart at chartDir and uploads it via the ChartMuseum API.
func (p *ChartMuseumPublisher) Push(chartDir, _ string) error {
	ch, err := helmchart.Load(chartDir)
	if err != nil {
		return fmt.Errorf("loading chart at %s: %w", chartDir, err)
	}

	tmpDir, err := os.MkdirTemp("", "helm-semver-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir) //nolint:errcheck

	pkg := action.NewPackage()
	pkg.Destination = tmpDir
	tgzPath, err := pkg.Run(chartDir, nil)
	if err != nil {
		return fmt.Errorf("packaging chart %s: %w", ch.Name(), err)
	}

	f, err := os.Open(tgzPath) // #nosec
	if err != nil {
		return fmt.Errorf("opening packaged chart: %w", err)
	}
	defer f.Close() //nolint:errcheck

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("chart", filepath.Base(tgzPath))
	if err != nil {
		return fmt.Errorf("creating form file: %w", err)
	}
	if _, err = io.Copy(part, f); err != nil {
		return fmt.Errorf("copying chart data: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("closing multipart writer: %w", err)
	}

	url := fmt.Sprintf("%s/api/charts", p.BaseURL)
	req, err := http.NewRequest(http.MethodPost, url, &body) //nolint:noctx
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if p.Username != "" {
		req.SetBasicAuth(p.Username, p.Password)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("POST %s: %w", url, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("chartmuseum returned HTTP %d for %s", resp.StatusCode, url)
	}

	return nil
}
