package cli

import (
	"fmt"

	"mgtt/internal/provider"
	"mgtt/internal/render"

	"github.com/spf13/cobra"
)

var providerCmd = &cobra.Command{
	Use:   "provider",
	Short: "Provider operations",
}

var providerInstallCmd = &cobra.Command{
	Use:   "install [names...]",
	Short: "Install one or more providers",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		for _, name := range args {
			p, err := provider.LoadEmbedded(name)
			if err != nil {
				return fmt.Errorf("provider %q: %w", name, err)
			}
			render.ProviderInstall(cmd.OutOrStdout(), p)
		}
		return nil
	},
}

func init() {
	providerCmd.AddCommand(providerInstallCmd)
	rootCmd.AddCommand(providerCmd)
}
