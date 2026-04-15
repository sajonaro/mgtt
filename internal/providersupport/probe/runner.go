package probe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"
)

const (
	defaultTimeout = 30 * time.Second
	maxStdoutBytes = 10 * 1024 * 1024
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
	args, err := buildArgs(cmd)
	if err != nil {
		return Result{}, err
	}

	timeout := cmd.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	c := exec.CommandContext(ctx, r.Binary, args...)
	var stderr strings.Builder
	c.Stderr = &stderr
	stdout, runErr := c.Output()

	if runErr != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return Result{}, fmt.Errorf("%w: runner %s exceeded %s", ErrTransient, r.Binary, timeout)
		}
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			return Result{}, ClassifyExit(exitErr.ExitCode(), stderr.String())
		}
		return Result{}, fmt.Errorf("%w: runner %s: %v", ErrEnv, r.Binary, runErr)
	}

	if len(stdout) > maxStdoutBytes {
		return Result{}, fmt.Errorf("%w: runner %s output %d bytes exceeds cap %d",
			ErrProtocol, r.Binary, len(stdout), maxStdoutBytes)
	}

	var rr struct {
		Value  any    `json:"value"`
		Raw    string `json:"raw"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(stdout, &rr); err != nil {
		return Result{}, fmt.Errorf("%w: parse runner %s output: %v", ErrProtocol, r.Binary, err)
	}
	if rr.Status == "" {
		rr.Status = StatusOk
	}
	return Result{Raw: rr.Raw, Parsed: rr.Value, Status: rr.Status}, nil
}

// buildArgs constructs the runner argv per the probe protocol:
//
//	probe <component> <fact> [--type T] [--<key> <value> ...]
//
// All Vars and Extra entries are passed as flags in alphabetical key order.
// Key collisions between Vars and Extra are a usage error — caller must
// resolve them before invoking Run.
func buildArgs(cmd Command) ([]string, error) {
	for k := range cmd.Extra {
		if _, conflict := cmd.Vars[k]; conflict {
			return nil, fmt.Errorf("%w: key %q present in both Vars and Extra", ErrUsage, k)
		}
	}
	merged := make(map[string]string, len(cmd.Vars)+len(cmd.Extra))
	for k, v := range cmd.Vars {
		if v != "" {
			merged[k] = v
		}
	}
	for k, v := range cmd.Extra {
		if v != "" {
			merged[k] = v
		}
	}
	keys := make([]string, 0, len(merged))
	for k := range merged {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	args := []string{"probe", cmd.Component, cmd.Fact}
	if cmd.Type != "" {
		args = append(args, "--type", cmd.Type)
	}
	for _, k := range keys {
		args = append(args, "--"+k, merged[k])
	}
	return args, nil
}

// Mux dispatches commands to a per-provider Executor, falling back to Default.
// Runners is keyed by provider name and typed as the Executor interface so
// any implementation (test fakes, alternate backends) plugs in uniformly.
type Mux struct {
	Default Executor
	Runners map[string]Executor
}

func (m *Mux) Run(ctx context.Context, cmd Command) (Result, error) {
	if r, ok := m.Runners[cmd.Provider]; ok {
		return r.Run(ctx, cmd)
	}
	return m.Default.Run(ctx, cmd)
}
