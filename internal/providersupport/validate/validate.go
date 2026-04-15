// Package validate runs correctness checks on a loaded provider.
//
// Static checks (always safe in CI):
//   - meta fields populated
//   - every fact has a probe.cmd
//   - default_active_state references a declared state
//   - auth.access.writes is "none" (or warn if any other value)
//   - meta.requires.mgtt is satisfied
//
// Live checks (require backend access; opt-in via --live in the CLI) are
// orchestrated separately and not part of this package.
package validate

import (
	"fmt"

	"github.com/mgt-tool/mgtt/internal/providersupport"
)

// Report holds the outcome of validation. OK reports whether any failures
// were recorded; warnings do not affect OK.
type Report struct {
	Passed   []string
	Warnings []string
	Failures []string
}

func (r Report) OK() bool { return len(r.Failures) == 0 }

// Static runs all checks that do not touch the backend.
func Static(p *providersupport.Provider) Report {
	var r Report

	if p.Meta.Name == "" {
		r.Failures = append(r.Failures, "meta.name is empty")
	}
	if p.Meta.Version == "" {
		r.Failures = append(r.Failures, "meta.version is empty")
	}
	if p.Meta.Command == "" {
		r.Failures = append(r.Failures, "meta.command is empty")
	}

	switch p.Auth.Access.Writes {
	case "":
		r.Failures = append(r.Failures,
			"auth.access.writes is not declared — must be \"none\" or an explicit scope")
	case "none":
		// good — declared read-only
	default:
		r.Warnings = append(r.Warnings, fmt.Sprintf(
			"auth.access.writes=%q — operators must confirm credentials match this scope",
			p.Auth.Access.Writes))
	}

	if err := p.CheckCompatible(); err != nil {
		r.Failures = append(r.Failures, err.Error())
	}

	for typeName, typ := range p.Types {
		if typ.DefaultActiveState != "" {
			found := false
			for _, s := range typ.States {
				if s.Name == typ.DefaultActiveState {
					found = true
					break
				}
			}
			if !found {
				r.Failures = append(r.Failures, fmt.Sprintf(
					"%s: default_active_state %q is not in declared states",
					typeName, typ.DefaultActiveState))
			}
		}

		for factName, f := range typ.Facts {
			if f.Probe.Cmd == "" {
				r.Failures = append(r.Failures, fmt.Sprintf(
					"%s/%s: probe.cmd is empty", typeName, factName))
			}
			if f.Probe.Parse == "" {
				r.Warnings = append(r.Warnings, fmt.Sprintf(
					"%s/%s: probe.parse empty (defaults to string)", typeName, factName))
			}
		}
	}

	if r.OK() && len(r.Warnings) == 0 {
		r.Passed = append(r.Passed, "static checks: ok")
	}
	return r
}
