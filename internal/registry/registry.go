// Package registry provides backends for publishing packaged Helm charts.
package registry

// Publisher is the interface implemented by all registry backends.
type Publisher interface {
	// Push packages the chart at chartDir and publishes it to the registry.
	Push(chartDir, version string) error
}
