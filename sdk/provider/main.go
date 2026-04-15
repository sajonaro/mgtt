package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// Version is set by providers via ldflags at build time.
var Version = "dev"

// Main is the standard entrypoint. Providers call it from main().
func Main(r *Registry) {
	code := Run(context.Background(), r, os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(code)
}

// Run is the testable core of Main. Returns the exit code per the probe
// protocol.
func Run(ctx context.Context, r *Registry, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: <runner> probe <name> <fact> [--type TYPE] [--<key> <value> ...]")
		return 1
	}
	switch args[0] {
	case "version":
		fmt.Fprintln(stdout, Version)
		return 0
	case "probe":
		// fall through
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
		return 1
	}
	if len(args) < 3 {
		fmt.Fprintln(stderr, "probe requires <name> and <fact>")
		return 1
	}

	req := Request{
		Name:      args[1],
		Fact:      args[2],
		Namespace: "default",
		Extra:     map[string]string{},
	}
	for i := 3; i+1 < len(args); i += 2 {
		key, val := args[i], args[i+1]
		if !strings.HasPrefix(key, "--") {
			fmt.Fprintf(stderr, "unexpected positional arg %q (flags must use --key value)\n", key)
			return 1
		}
		k := strings.TrimPrefix(key, "--")
		switch k {
		case "type":
			req.Type = val
		case "namespace":
			req.Namespace = val
			req.Extra[k] = val
		default:
			req.Extra[k] = val
		}
	}

	res, err := r.Probe(ctx, req)
	if err != nil {
		return exitFor(err, stderr)
	}
	if err := json.NewEncoder(stdout).Encode(res); err != nil {
		fmt.Fprintln(stderr, err)
		return 5
	}
	return 0
}

func exitFor(err error, stderr io.Writer) int {
	fmt.Fprintln(stderr, err)
	switch {
	case errors.Is(err, ErrUsage):
		return 1
	case errors.Is(err, ErrEnv):
		return 2
	case errors.Is(err, ErrForbidden):
		return 3
	case errors.Is(err, ErrTransient):
		return 4
	case errors.Is(err, ErrProtocol):
		return 5
	}
	return 1
}
