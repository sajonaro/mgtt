package cli

import (
	"os"

	"mgtt/internal/model"
	"mgtt/internal/providersupport"
	"mgtt/internal/render"

	"github.com/spf13/cobra"
)

var modelCmd = &cobra.Command{
	Use:   "model",
	Short: "Model operations",
}

var modelValidateCmd = &cobra.Command{
	Use:   "validate [path]",
	Short: "Validate system.model.yaml",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "system.model.yaml"
		if len(args) > 0 {
			path = args[0]
		}

		m, err := model.Load(path)
		if err != nil {
			return err
		}

		// Load embedded providers and build registry for type resolution.
		reg := providersupport.NewRegistry()
		for _, name := range providersupport.ListEmbedded() {
			p, err := providersupport.LoadEmbedded(name)
			if err == nil {
				reg.Register(p)
			}
		}

		result := model.Validate(m, reg)

		// Build depCounts: map component name → number of direct dependencies.
		// Use -1 to signal "no deps but has a healthy override".
		depCounts := make(map[string]int, len(m.Components))
		for _, name := range m.Order {
			comp := m.Components[name]
			count := 0
			for _, dep := range comp.Depends {
				count += len(dep.On)
			}
			if count == 0 && len(comp.HealthyRaw) > 0 {
				depCounts[name] = -1
			} else {
				depCounts[name] = count
			}
		}

		render.ModelValidate(cmd.OutOrStdout(), result, m.Order, depCounts)

		if result.HasErrors() {
			os.Exit(1)
		}
		return nil
	},
}

func init() {
	modelCmd.AddCommand(modelValidateCmd)
	rootCmd.AddCommand(modelCmd)
}
