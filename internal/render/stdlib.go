package render

import (
	"fmt"
	"io"
	"strings"

	"mgtt/internal/provider"
)

// stdlibOrder is the canonical display order for stdlib types.
var stdlibOrder = []string{
	"int", "float", "bool", "string",
	"duration", "bytes", "ratio", "percentage", "count", "timestamp",
}

// StdlibLs writes a summary table of all stdlib types to w.
//
// Example output:
//
//	int         base: int    unit: ~              range: ~
//	float       base: float  unit: ~              range: ~
//	...
func StdlibLs(w io.Writer) {
	for _, name := range stdlibOrder {
		dt, ok := provider.Stdlib[name]
		if !ok {
			continue
		}
		units := "~"
		if len(dt.Units) > 0 {
			units = strings.Join(dt.Units, "|")
		}
		rangeStr := formatRange(dt.Range)
		fmt.Fprintf(w, "  %-12s  base: %-7s  unit: %-20s  range: %s\n",
			name, dt.Base, units, rangeStr)
	}
}

// StdlibInspect writes detailed information about a single stdlib type to w.
func StdlibInspect(w io.Writer, name string) {
	dt, ok := provider.Stdlib[name]
	if !ok {
		fmt.Fprintf(w, "  stdlib type %q not found\n", name)
		return
	}

	fmt.Fprintf(w, "  name:  %s\n", dt.Name)
	fmt.Fprintf(w, "  base:  %s\n", dt.Base)

	if len(dt.Units) > 0 {
		fmt.Fprintf(w, "  units: %s\n", strings.Join(dt.Units, ", "))
	} else {
		fmt.Fprintf(w, "  units: ~\n")
	}

	fmt.Fprintf(w, "  range: %s\n", formatRange(dt.Range))
}

// formatRange returns a human-readable range string.
//   - nil Range → "~"
//   - Min only  → "0.."
//   - Both      → "0..1"
func formatRange(r *provider.Range) string {
	if r == nil {
		return "~"
	}
	if r.Min != nil && r.Max != nil {
		return fmt.Sprintf("%g..%g", *r.Min, *r.Max)
	}
	if r.Min != nil {
		return fmt.Sprintf("%g..", *r.Min)
	}
	if r.Max != nil {
		return fmt.Sprintf("..%g", *r.Max)
	}
	return "~"
}
