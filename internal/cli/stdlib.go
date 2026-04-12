package cli

import (
	"fmt"

	"mgtt/internal/providersupport"
	"mgtt/internal/render"

	"github.com/spf13/cobra"
)

var stdlibCmd = &cobra.Command{
	Use:   "stdlib",
	Short: "Stdlib type operations",
}

var stdlibLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List all stdlib types",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		render.StdlibLs(cmd.OutOrStdout())
		return nil
	},
}

var stdlibInspectCmd = &cobra.Command{
	Use:   "inspect <type>",
	Short: "Inspect a stdlib type",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if _, ok := providersupport.Stdlib[name]; !ok {
			return fmt.Errorf("stdlib type %q not found", name)
		}
		render.StdlibInspect(cmd.OutOrStdout(), name)
		return nil
	},
}

func init() {
	stdlibCmd.AddCommand(stdlibLsCmd)
	stdlibCmd.AddCommand(stdlibInspectCmd)
	rootCmd.AddCommand(stdlibCmd)
}
