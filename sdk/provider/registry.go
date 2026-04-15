package provider

import (
	"context"
	"errors"
	"fmt"
)

// Request is the typed input handed to a ProbeFn. Type is reserved by the
// protocol (it selects the registered fact set). Everything else — including
// backend-specific keys like "namespace", "region", "cluster" — lives in
// Extra, opaque to core. Providers use Request.Namespace() etc. accessors
// when they need sugar, but core never privileges any key.
type Request struct {
	Type  string
	Name  string
	Fact  string
	Extra map[string]string // every --<key> <value> pair from the runner argv (except --type)
}

// Namespace is a convenience accessor for Extra["namespace"]. Returns ""
// when the flag was not provided. Providers that want a different default
// should apply it themselves — core does NOT inject one.
func (r Request) Namespace() string { return r.Extra["namespace"] }

// ProbeFn implements one fact for one type.
type ProbeFn func(ctx context.Context, req Request) (Result, error)

// Registry maps a type name to its set of fact probe functions.
type Registry struct {
	types map[string]map[string]ProbeFn
}

// NewRegistry creates an empty registry. Providers register each type from
// main() before calling Main(reg).
func NewRegistry() *Registry { return &Registry{types: map[string]map[string]ProbeFn{}} }

// Register adds (or replaces) a type's fact set.
func (r *Registry) Register(typ string, facts map[string]ProbeFn) {
	r.types[typ] = facts
}

// Probe dispatches to the registered ProbeFn. Errors that wrap ErrNotFound
// are translated to Result{Status: not_found} per the probe protocol.
func (r *Registry) Probe(ctx context.Context, req Request) (Result, error) {
	facts, ok := r.types[req.Type]
	if !ok {
		return Result{}, fmt.Errorf("%w: unknown type %q", ErrUsage, req.Type)
	}
	fn, ok := facts[req.Fact]
	if !ok {
		return Result{}, fmt.Errorf("%w: type %q has no fact %q", ErrUsage, req.Type, req.Fact)
	}
	res, err := fn(ctx, req)
	if errors.Is(err, ErrNotFound) {
		return NotFound(), nil
	}
	if err != nil {
		return Result{}, err
	}
	if res.Status == "" {
		res.Status = StatusOk
	}
	return res, nil
}

// Types returns registered type names — used by validate tooling.
func (r *Registry) Types() []string {
	out := make([]string, 0, len(r.types))
	for k := range r.types {
		out = append(out, k)
	}
	return out
}

// Facts returns registered fact names for a type — used by validate tooling.
func (r *Registry) Facts(typ string) []string {
	facts := r.types[typ]
	out := make([]string, 0, len(facts))
	for k := range facts {
		out = append(out, k)
	}
	return out
}
