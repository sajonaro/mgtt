package render

import (
	"fmt"
	"io"
	"strings"

	"mgtt/internal/model"
)

// ModelValidate writes a human-readable validation report to w.
//
// components is the ordered list of component names to display.
// depCounts maps component name → number of direct dependencies.
func ModelValidate(w io.Writer, result *model.ValidationResult, components []string, depCounts map[string]int) {
	// Index errors by component name for quick lookup.
	compErrors := make(map[string][]model.ValidationError)
	var globalErrors []model.ValidationError
	for _, e := range result.Errors {
		if e.Component == "" {
			globalErrors = append(globalErrors, e)
		} else {
			compErrors[e.Component] = append(compErrors[e.Component], e)
		}
	}

	// Determine the longest component name for alignment.
	maxLen := 0
	for _, name := range components {
		if len(name) > maxLen {
			maxLen = len(name)
		}
	}

	// Print one line per component.
	for _, name := range components {
		errs := compErrors[name]
		if len(errs) == 0 {
			// Component is valid — pick the right description.
			desc := componentDesc(name, depCounts[name])
			fmt.Fprintf(w, "  %s %-*s  %s\n", Checkmark(true), maxLen, name, desc)
		} else {
			// One line per error for this component.
			for i, e := range errs {
				if i == 0 {
					fmt.Fprintf(w, "  %s %-*s  %s\n", Checkmark(false), maxLen, name, e.Message)
				} else {
					fmt.Fprintf(w, "  %s %-*s  %s\n", Checkmark(false), maxLen, "", e.Message)
				}
				if e.Suggestion != "" {
					indent := strings.Repeat(" ", 2+1+1+maxLen+2) // "  ✗ " + padding + "  "
					fmt.Fprintf(w, "%sdid you mean %q?\n", indent, e.Suggestion)
				}
			}
		}
	}

	// Print global errors (cycles etc.) — not tied to a component.
	for _, e := range globalErrors {
		fmt.Fprintf(w, "  %s %s\n", Checkmark(false), e.Message)
	}

	// Blank line before summary.
	fmt.Fprintln(w)

	errCount := len(result.Errors)
	warnCount := len(result.Warnings)
	fmt.Fprintf(w, "  %s · %s · %s\n",
		Pluralize(len(components), "component", "components"),
		Pluralize(errCount, "error", "errors"),
		Pluralize(warnCount, "warning", "warnings"),
	)
}

// componentDesc returns the per-component status description.
//
// Convention for depCount:
//   - -1 : no dependencies, but component has a healthy-override (HealthyRaw)
//   - 0  : no dependencies, no healthy-override
//   -  N : N direct dependencies
func componentDesc(_ string, depCount int) string {
	if depCount < 0 {
		return "healthy override valid"
	}
	if depCount == 0 {
		return "no dependencies"
	}
	return Pluralize(depCount, "dependency valid", "dependencies valid")
}
