package cli

import (
	"fmt"
	"io"

	"github.com/mgt-tool/mgtt/internal/providersupport"
	"github.com/mgt-tool/mgtt/internal/providersupport/validate"

	"github.com/spf13/cobra"
)

var providerValidateCmd = &cobra.Command{
	Use:   "validate <name>",
	Short: "Run static correctness checks on a provider",
	Long: `Validate a provider's manifest against the probe protocol.

Static checks (always safe):
  - meta.name, version, command populated
  - auth.access.writes declared (must be "none" or explicit)
  - meta.requires.mgtt is satisfied by the running mgtt
  - every fact has probe.cmd; warns if probe.parse is missing
  - default_active_state references a declared state

Live checks against a real backend (--live) are scoped per-provider and
not yet implemented in core; provider repos run their own --live in CI.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := providersupport.LoadEmbedded(args[0])
		if err != nil {
			return fmt.Errorf("provider %q: %w", args[0], err)
		}
		rep := validate.Static(p)
		renderReport(cmd.OutOrStdout(), rep)
		if !rep.OK() {
			return fmt.Errorf("validation failed: %d failures", len(rep.Failures))
		}
		return nil
	},
}

func init() {
	providerCmd.AddCommand(providerValidateCmd)
}

func renderReport(w io.Writer, r validate.Report) {
	for _, p := range r.Passed {
		fmt.Fprintln(w, "PASS  "+p)
	}
	for _, msg := range r.Warnings {
		fmt.Fprintln(w, "WARN  "+msg)
	}
	for _, msg := range r.Failures {
		fmt.Fprintln(w, "FAIL  "+msg)
	}
}
