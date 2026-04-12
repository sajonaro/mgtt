package main

import (
	"fmt"
	"os"

	root "mgtt"
	"mgtt/internal/cli"
	"mgtt/internal/provider"
)

func main() {
	provider.Embedded = root.EmbeddedProviders

	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "mgtt: internal error: %v\nThis is a bug. Please report it at https://github.com/mgtt/mgtt/issues\n", r)
			os.Exit(3)
		}
	}()
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
