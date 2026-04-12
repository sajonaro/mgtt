package cli

import (
	"fmt"

	"mgtt/internal/providersupport"
	"mgtt/internal/render"

	"github.com/spf13/cobra"
)

var providerLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List available providers",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		names := providersupport.ListEmbedded()
		var providers []*providersupport.Provider
		for _, name := range names {
			p, err := providersupport.LoadEmbedded(name)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not load provider %q: %v\n", name, err)
				continue
			}
			providers = append(providers, p)
		}
		render.ProviderLs(cmd.OutOrStdout(), providers)
		return nil
	},
}

func init() {
	providerCmd.AddCommand(providerLsCmd)
}
