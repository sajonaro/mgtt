package probe

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

// ExternalRunner invokes a provider's runner binary and parses its JSON
// result. It implements Executor so it can be composed in a Mux.
type ExternalRunner struct {
	Binary string
}

func NewExternalRunner(binary string) *ExternalRunner {
	return &ExternalRunner{Binary: binary}
}

// Run implements Executor.
func (r *ExternalRunner) Run(ctx context.Context, cmd Command) (Result, error) {
	args := []string{"probe", cmd.Component, cmd.Fact}
	if ns := cmd.Vars["namespace"]; ns != "" {
		args = append(args, "--namespace", ns)
	}
	if cmd.Type != "" {
		args = append(args, "--type", cmd.Type)
	}

	out, err := exec.CommandContext(ctx, r.Binary, args...).Output()
	if err != nil {
		return Result{}, fmt.Errorf("runner %s: %w", r.Binary, err)
	}

	var rr struct {
		Value any    `json:"value"`
		Raw   string `json:"raw"`
	}
	if err := json.Unmarshal(out, &rr); err != nil {
		return Result{}, fmt.Errorf("runner %s: parse output: %w", r.Binary, err)
	}
	return Result{Raw: rr.Raw, Parsed: rr.Value}, nil
}

// Mux routes probe commands to a per-provider runner when one is registered,
// falling back to Default otherwise.
type Mux struct {
	Default Executor
	Runners map[string]*ExternalRunner
}

func (m *Mux) Run(ctx context.Context, cmd Command) (Result, error) {
	if r, ok := m.Runners[cmd.Provider]; ok {
		return r.Run(ctx, cmd)
	}
	return m.Default.Run(ctx, cmd)
}
