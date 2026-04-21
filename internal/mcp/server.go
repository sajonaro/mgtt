// Package mcp exposes mgtt's constraint engine as an MCP service. See
// docs/superpowers/specs/2026-04-22-llm-support-design.md for the full
// contract. The handler reuses the CLI's engine path — engine.Plan,
// scenarios, facts, incident — via thin tool wrappers.
package mcp

import (
	"errors"

	_ "github.com/mark3labs/mcp-go/server" // holds the dep pinned until Task 2 uses it for real
)

type Config struct {
	Version               string // populated by the CLI from cli.version at startup
	HTTP                  bool
	Listen                string
	TokenEnv              string
	ReadonlyOnly          bool
	OnWrite               string // "pause" | "run" | "fail"
	MaxExecutePerIncident int
	ProbeTimeoutSeconds   int
}

// Run boots the MCP server with the given config. Blocks until the
// transport closes (stdin EOF for stdio, SIGINT for HTTP).
func Run(cfg Config) error {
	return errors.New("mgtt mcp serve: not yet implemented")
}
