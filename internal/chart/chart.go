// Package chart reads and writes Helm Chart.yaml files.
package chart

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Metadata holds the fields from Chart.yaml that helm-semver cares about.
// Additional fields are preserved through the raw node tree.
type Metadata struct {
	Name       string `yaml:"name"`
	Version    string `yaml:"version"`
	AppVersion string `yaml:"appVersion,omitempty"`
}

// Load reads a Chart.yaml file and returns its metadata.
func Load(path string) (*Metadata, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path comes from controlled CLI input
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var m Metadata
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	if m.Name == "" {
		return nil, fmt.Errorf("%s: missing required field 'name'", path)
	}
	if m.Version == "" {
		return nil, fmt.Errorf("%s: missing required field 'version'", path)
	}

	return &m, nil
}

// BumpVersion rewrites only the `version:` line in a Chart.yaml file,
// preserving all other content (comments, field order, appVersion, etc.).
func BumpVersion(path, newVersion string) error {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	// Parse into a generic node tree to preserve comments and ordering.
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}

	if err := setMappingValue(&doc, "version", newVersion); err != nil {
		return fmt.Errorf("updating version in %s: %w", path, err)
	}

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("marshalling %s: %w", path, err)
	}

	if err := os.WriteFile(path, out, 0o644); err != nil { //nolint:gosec
		return fmt.Errorf("writing %s: %w", path, err)
	}

	return nil
}

// setMappingValue finds the scalar value node for key in a YAML document node
// and replaces its value. Returns an error if the key is not found.
func setMappingValue(doc *yaml.Node, key, value string) error {
	// doc is a Document node; its first Content child is the Mapping.
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		return setMappingValue(doc.Content[0], key, value)
	}
	if doc.Kind != yaml.MappingNode {
		return fmt.Errorf("expected mapping node, got kind %d", doc.Kind)
	}

	for i := 0; i+1 < len(doc.Content); i += 2 {
		keyNode := doc.Content[i]
		valNode := doc.Content[i+1]
		if keyNode.Value == key {
			valNode.Value = value
			return nil
		}
	}

	return fmt.Errorf("key %q not found in mapping", key)
}
