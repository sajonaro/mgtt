package mgtt

import "embed"

//go:embed providers/*/provider.yaml
var EmbeddedProviders embed.FS
