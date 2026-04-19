package model_test

import (
	"strings"
	"testing"

	"github.com/mgt-tool/mgtt/internal/model"
	"github.com/mgt-tool/mgtt/internal/providersupport"
)

// TestRender_MinimalTwoNode covers the happy path: a two-component model
// with one dependency renders an H1 header, a mermaid block opener, one
// subgraph (or flat since both components share a provider), node
// declarations, and an edge.
func TestRender_MinimalTwoNode(t *testing.T) {
	m := &model.Model{
		Meta: model.Meta{Name: "demo", Version: "1.0", Providers: []string{"k8s"}},
		Components: map[string]*model.Component{
			"frontend": {Name: "frontend", Type: "deployment", Depends: []model.Dependency{{On: []string{"backend"}}}},
			"backend":  {Name: "backend", Type: "deployment"},
		},
		Order: []string{"frontend", "backend"},
	}
	reg := providersupport.NewRegistry()
	reg.Register(&providersupport.Provider{
		Meta:  providersupport.ProviderMeta{Name: "k8s"},
		Types: map[string]*providersupport.Type{"deployment": {Name: "deployment"}},
	})
	installed := []model.InstalledProvider{
		{Name: "k8s", Namespace: "", Version: "1.0.0"},
	}

	got, err := model.Render(m, reg, installed)
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}

	for _, want := range []string{
		"# demo — dependency graph",
		"```mermaid",
		"graph LR",
		`frontend["frontend<br/>deployment"]`,
		`backend["backend<br/>deployment"]`,
		"frontend --> backend",
		"\n```\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output:\n%s", want, got)
		}
	}
}
