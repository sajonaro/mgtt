package model

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mgt-tool/mgtt/internal/providersupport"
)

// Render emits the markdown+mermaid body for m. The registry is used to
// resolve each component's type to its owning provider (bare name); the
// installed list supplies the Namespace so the owning name can be
// promoted to an FQN for subgraph grouping. Pure function — no file
// I/O, no goroutines, no globals.
func Render(m *Model, reg *providersupport.Registry, installed []InstalledProvider) (string, error) {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s — dependency graph\n\n", m.Meta.Name)
	sb.WriteString("```mermaid\ngraph LR\n")

	names := sortedComponentNames(m)
	for _, n := range names {
		c := m.Components[n]
		label := fmt.Sprintf("%s<br/>%s", n, c.Type)
		fmt.Fprintf(&sb, "  %s[%q]\n", n, label)
	}

	// Edges. Dependency.On is []string — a single clause can name
	// multiple upstreams, each becoming its own edge.
	type edge struct{ from, to string }
	var edges []edge
	for _, n := range names {
		c := m.Components[n]
		for _, d := range c.Depends {
			for _, on := range d.On {
				edges = append(edges, edge{from: n, to: on})
			}
		}
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].from != edges[j].from {
			return edges[i].from < edges[j].from
		}
		return edges[i].to < edges[j].to
	})
	for _, e := range edges {
		fmt.Fprintf(&sb, "  %s --> %s\n", e.from, e.to)
	}

	sb.WriteString("```\n")
	return sb.String(), nil
}

func sortedComponentNames(m *Model) []string {
	names := make([]string, 0, len(m.Components))
	for n := range m.Components {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
