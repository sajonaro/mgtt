package cli

import (
	"fmt"

	"mgtt/internal/providersupport"
	"mgtt/internal/render"

	"github.com/spf13/cobra"
)

var providerInspectCmd = &cobra.Command{
	Use:   "inspect <name> [type]",
	Short: "Inspect a provider or a specific type within it",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		typeName := ""
		if len(args) == 2 {
			typeName = args[1]
		}

		p, err := providersupport.LoadEmbedded(name)
		if err != nil {
			return fmt.Errorf("provider %q: %w", name, err)
		}

		render.ProviderInspect(cmd.OutOrStdout(), p, typeName)
		return nil
	},
}

func init() {
	providerCmd.AddCommand(providerInspectCmd)
}
