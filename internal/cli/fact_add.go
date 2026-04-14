package cli

import (
	"fmt"
	"time"

	"github.com/mgt-tool/mgtt/internal/expr"
	"github.com/mgt-tool/mgtt/internal/facts"
	"github.com/mgt-tool/mgtt/internal/incident"

	"github.com/spf13/cobra"
)

var factCmd = &cobra.Command{
	Use:   "fact",
	Short: "Manage facts",
}

var factAddNote string

var factAddCmd = &cobra.Command{
	Use:   "add <component> <key> <value>",
	Short: "Add a fact to the current incident",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		component := args[0]
		key := args[1]
		rawValue := args[2]

		inc, err := incident.Current()
		if err != nil {
			return fmt.Errorf("no active incident: %w", err)
		}

		value := expr.InferValue(rawValue)
		f := facts.Fact{
			Key:       key,
			Value:     value,
			Collector: "manual",
			At:        time.Now(),
			Note:      factAddNote,
		}

		if err := inc.Store.AppendAndSave(component, f); err != nil {
			return fmt.Errorf("saving fact: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "  %s %s.%s = %v\n", "added", component, key, value)
		return nil
	},
}

func init() {
	factAddCmd.Flags().StringVar(&factAddNote, "note", "", "optional note for the fact")
	factCmd.AddCommand(factAddCmd)
	rootCmd.AddCommand(factCmd)
}
