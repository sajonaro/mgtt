package render

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"mgtt/internal/providersupport"
)

// ProviderInstall writes a one-line confirmation for a successfully installed
// provider to w.
//
// Example:
//
//	  ✓ kubernetes  v1.0.0  auth: environment  access: kubectl read-only
func ProviderInstall(w io.Writer, p *providersupport.Provider) {
	fmt.Fprintf(w, "  %s %-12s  v%s  auth: %s  access: %s\n",
		Checkmark(true),
		p.Meta.Name,
		p.Meta.Version,
		p.Auth.Strategy,
		p.Auth.Access.Probes,
	)
}

// ProviderLs writes one line per provider with a checkmark, name, version,
// and description to w.
func ProviderLs(w io.Writer, providers []*providersupport.Provider) {
	if len(providers) == 0 {
		fmt.Fprintln(w, "  no providers installed")
		return
	}

	// Determine column widths.
	maxName := 0
	maxVersion := 0
	for _, p := range providers {
		if n := len(p.Meta.Name); n > maxName {
			maxName = n
		}
		v := "v" + p.Meta.Version
		if n := len(v); n > maxVersion {
			maxVersion = n
		}
	}

	for _, p := range providers {
		ver := "v" + p.Meta.Version
		fmt.Fprintf(w, "  %s %-*s  %-*s  %s\n",
			Checkmark(true),
			maxName, p.Meta.Name,
			maxVersion, ver,
			p.Meta.Description,
		)
	}
}

// ProviderInspect writes provider details to w. When typeName is empty, an
// overview is shown (name, version, description, list of types). When typeName
// is non-empty, detailed type information is shown (facts, healthy conditions,
// states, default_active_state, failure_modes).
func ProviderInspect(w io.Writer, p *providersupport.Provider, typeName string) {
	if typeName == "" {
		providerOverview(w, p)
		return
	}
	t, ok := p.Types[typeName]
	if !ok {
		fmt.Fprintf(w, "  type %q not found in provider %q\n", typeName, p.Meta.Name)
		return
	}
	typeDetail(w, p, t)
}

// providerOverview renders the provider summary view.
func providerOverview(w io.Writer, p *providersupport.Provider) {
	fmt.Fprintf(w, "  provider:    %s\n", p.Meta.Name)
	fmt.Fprintf(w, "  version:     v%s\n", p.Meta.Version)
	fmt.Fprintf(w, "  description: %s\n", p.Meta.Description)
	fmt.Fprintf(w, "  auth:        %s\n", p.Auth.Strategy)
	fmt.Fprintf(w, "  access:      %s\n", p.Auth.Access.Probes)
	fmt.Fprintln(w)

	// Sorted type names.
	typeNames := make([]string, 0, len(p.Types))
	for k := range p.Types {
		typeNames = append(typeNames, k)
	}
	sort.Strings(typeNames)

	fmt.Fprintf(w, "  types (%d):\n", len(typeNames))
	for _, name := range typeNames {
		t := p.Types[name]
		desc := t.Description
		if desc == "" {
			desc = "-"
		}
		fmt.Fprintf(w, "    %-20s  %s\n", name, desc)
	}
}

// typeDetail renders the detailed view for a single type.
func typeDetail(w io.Writer, p *providersupport.Provider, t *providersupport.Type) {
	fmt.Fprintf(w, "  provider:  %s\n", p.Meta.Name)
	fmt.Fprintf(w, "  type:      %s\n", t.Name)
	if t.Description != "" {
		fmt.Fprintf(w, "  desc:      %s\n", t.Description)
	}
	fmt.Fprintln(w)

	// Facts — sorted by name.
	if len(t.Facts) > 0 {
		factNames := make([]string, 0, len(t.Facts))
		for k := range t.Facts {
			factNames = append(factNames, k)
		}
		sort.Strings(factNames)

		fmt.Fprintln(w, "  facts:")
		for _, name := range factNames {
			fs := t.Facts[name]
			ttl := fs.TTL.String()
			if fs.TTL == 0 {
				ttl = "~"
			}
			cost := fs.Probe.Cost
			if cost == "" {
				cost = "~"
			}
			fmt.Fprintf(w, "    %-20s  type: %-14s  ttl: %-6s  cost: %s\n",
				name, fs.TypeName, ttl, cost)
		}
		fmt.Fprintln(w)
	}

	// Healthy conditions.
	if len(t.HealthyRaw) > 0 {
		fmt.Fprintln(w, "  healthy:")
		for _, cond := range t.HealthyRaw {
			fmt.Fprintf(w, "    - %s\n", cond)
		}
		fmt.Fprintln(w)
	}

	// States (in declaration order).
	if len(t.States) > 0 {
		fmt.Fprintln(w, "  states:")
		for _, s := range t.States {
			marker := " "
			if s.Name == t.DefaultActiveState {
				marker = "*"
			}
			desc := s.Description
			if desc == "" {
				desc = "-"
			}
			fmt.Fprintf(w, "   %s %-20s  when: %-40s  desc: %s\n",
				marker, s.Name, s.WhenRaw, desc)
		}
		fmt.Fprintln(w)
	}

	// Failure modes — sorted by state name.
	if len(t.FailureModes) > 0 {
		fmStates := make([]string, 0, len(t.FailureModes))
		for k := range t.FailureModes {
			fmStates = append(fmStates, k)
		}
		sort.Strings(fmStates)

		fmt.Fprintln(w, "  failure_modes:")
		for _, state := range fmStates {
			causes := t.FailureModes[state]
			fmt.Fprintf(w, "    %-20s  can_cause: %s\n", state, strings.Join(causes, ", "))
		}
	}
}
