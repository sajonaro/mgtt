package provider

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Embedded holds the embedded provider filesystem. It is wired by the main
// package to the root-level EmbeddedProviders embed.FS variable.
var Embedded embed.FS

// EmbeddedRoot is the subdirectory within Embedded that contains providers.
var EmbeddedRoot string = "providers"

// LoadEmbedded loads a provider by name. It checks $MGTT_HOME first, then
// falls back to the embedded filesystem.
func LoadEmbedded(name string) (*Provider, error) {
	// Check $MGTT_HOME override first.
	if home := os.Getenv("MGTT_HOME"); home != "" {
		path := filepath.Join(home, "providers", name, "provider.yaml")
		if data, err := os.ReadFile(path); err == nil {
			return LoadFromBytes(data)
		}
	}

	// Fall back to embedded filesystem.
	data, err := Embedded.ReadFile(filepath.Join(EmbeddedRoot, name, "provider.yaml"))
	if err != nil {
		return nil, fmt.Errorf("provider %q not found", name)
	}
	return LoadFromBytes(data)
}

// ListEmbedded returns the names of all providers available in the embedded
// filesystem (or $MGTT_HOME if set).
func ListEmbedded() []string {
	var names []string

	if home := os.Getenv("MGTT_HOME"); home != "" {
		provDir := filepath.Join(home, "providers")
		entries, err := os.ReadDir(provDir)
		if err == nil {
			for _, e := range entries {
				if e.IsDir() {
					yamlPath := filepath.Join(provDir, e.Name(), "provider.yaml")
					if _, err := os.Stat(yamlPath); err == nil {
						names = append(names, e.Name())
					}
				}
			}
			return names
		}
	}

	// Walk the embedded FS.
	entries, err := Embedded.ReadDir(EmbeddedRoot)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if e.IsDir() {
			yamlPath := EmbeddedRoot + "/" + e.Name() + "/provider.yaml"
			if _, err := Embedded.Open(yamlPath); err == nil {
				name := e.Name()
				// Strip any path prefix.
				if idx := strings.LastIndex(name, "/"); idx >= 0 {
					name = name[idx+1:]
				}
				names = append(names, name)
			}
		}
	}
	return names
}
