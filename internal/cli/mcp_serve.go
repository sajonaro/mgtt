package cli

import (
	"github.com/mgt-tool/mgtt/internal/mcp"
	"github.com/spf13/cobra"
)

var mcpServeFlags mcp.Config

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "MCP server for LLM agents",
}

var mcpServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the MCP server (stdio by default, HTTP with --http)",
	RunE: func(cmd *cobra.Command, args []string) error {
		mcpServeFlags.Version = version // the package-level version var from internal/cli/root.go
		return mcp.Run(mcpServeFlags)
	},
}

func init() {
	mcpServeCmd.Flags().BoolVar(&mcpServeFlags.HTTP, "http", false, "run streamable HTTP transport instead of stdio")
	mcpServeCmd.Flags().StringVar(&mcpServeFlags.Listen, "listen", ":8080", "listen address for HTTP mode")
	mcpServeCmd.Flags().StringVar(&mcpServeFlags.TokenEnv, "token-env", "", "env var name holding the bearer token (required in HTTP mode)")
	mcpServeCmd.Flags().BoolVar(&mcpServeFlags.ReadonlyOnly, "readonly-only", false, "reject probes whose provider does not declare read_only: true")
	mcpServeCmd.Flags().StringVar(&mcpServeFlags.OnWrite, "on-write", "run", "behavior when a write probe is next: pause | run | fail")
	mcpServeCmd.Flags().IntVar(&mcpServeFlags.MaxExecutePerIncident, "max-execute-per-incident", 50, "rate limit for executed probes per incident")
	mcpServeCmd.Flags().IntVar(&mcpServeFlags.ProbeTimeoutSeconds, "probe-timeout", 30, "per-probe timeout in seconds (max 300)")

	mcpCmd.AddCommand(mcpServeCmd)
	rootCmd.AddCommand(mcpCmd)
}
