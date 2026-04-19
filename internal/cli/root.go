package cli

import (
	"runtime/debug"

	"github.com/spf13/cobra"
)

// version is the string shown by `mgtt version`. It carries the value
// baked in by the Makefile via ldflags (-X ...cli.version=$(VERSION)),
// which is what `make build` and the docker image do. When the binary
// is produced another way — `go build ./cmd/mgtt`, `go install
// github.com/mgt-tool/mgtt/cmd/mgtt@v0.2.0`, or `... @main` — the
// fallback in init() below populates it from the Go toolchain's
// recorded build info (module version or VCS revision).
var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "mgtt",
	Short: "Model Guided Troubleshooting Tool",
}

func init() {
	populateVersionFromBuildInfo()
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Println("mgtt version " + version)
		},
	})
}

// populateVersionFromBuildInfo is a no-op when ldflags already set
// version to a real value; otherwise it tries (a) the module version
// (works for `go install ...@<tag>` and for pseudo-versions like
// `v0.1.5-0.20260419…-38d55eb4649f` from `@main`), then (b) the VCS
// revision from local-build metadata (works for `go build` from a
// checkout). Falls through to leave version == "dev" when neither is
// available (e.g. built outside Go modules).
func populateVersionFromBuildInfo() {
	if version != "dev" {
		return // ldflags set a real value; trust it
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
		version = v
		return
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" && len(s.Value) >= 12 {
			version = "devel+" + s.Value[:12]
			return
		}
	}
}

func Execute() error {
	return rootCmd.Execute()
}

func RootCmd() *cobra.Command {
	return rootCmd
}
