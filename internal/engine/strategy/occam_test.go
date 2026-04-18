package strategy

import (
	"testing"
	"time"

	"github.com/mgt-tool/mgtt/internal/expr"
	"github.com/mgt-tool/mgtt/internal/facts"
	"github.com/mgt-tool/mgtt/internal/model"
	"github.com/mgt-tool/mgtt/internal/providersupport"
	"github.com/mgt-tool/mgtt/internal/scenarios"
)

// threeCompModel builds a three-component model (web → api → db) all
// backed by the same provider "p" with a single-state single-fact type
// per component. Used as the fixture for occam tests.
func threeCompModel(t *testing.T) (*model.Model, *providersupport.Registry) {
	t.Helper()
	mk := func(name string) *providersupport.Type {
		return &providersupport.Type{
			Name: name,
			Facts: map[string]*providersupport.FactSpec{
				"status": {Probe: providersupport.ProbeDef{Cmd: name + "-status", Cost: "cheap", Access: "read"}},
			},
			States: []providersupport.StateDef{
				{Name: "down", When: expr.CmpNode{Fact: "status", Op: expr.OpEq, Value: "down"}},
			},
		}
	}
	prov := &providersupport.Provider{
		Meta: providersupport.ProviderMeta{Name: "p"},
		Types: map[string]*providersupport.Type{
			"web": mk("web"),
			"api": mk("api"),
			"db":  mk("db"),
		},
	}
	reg := providersupport.NewRegistry()
	reg.Register(prov)

	m := &model.Model{
		Meta: model.Meta{Providers: []string{"p"}},
		Components: map[string]*model.Component{
			"web": {Name: "web", Type: "web", Depends: []model.Dependency{{On: []string{"api"}}}},
			"api": {Name: "api", Type: "api", Depends: []model.Dependency{{On: []string{"db"}}}},
			"db":  {Name: "db", Type: "db"},
		},
		Order: []string{"web", "api", "db"},
	}
	m.BuildGraph()
	return m, reg
}

func TestOccam_Name(t *testing.T) {
	if Occam().Name() != "occam" {
		t.Errorf("want occam; got %s", Occam().Name())
	}
}

func TestOccam_PicksShortestScenarioFirst(t *testing.T) {
	m, reg := threeCompModel(t)
	store := facts.NewInMemory()

	scs := []scenarios.Scenario{
		{
			ID: "long",
			Chain: []scenarios.Step{
				{Component: "web", State: "down"},
				{Component: "api", State: "down"},
				{Component: "db", State: "down"},
			},
		},
		{
			ID:    "short",
			Chain: []scenarios.Step{{Component: "web", State: "down"}},
		},
	}

	dec := Occam().SuggestProbe(Input{Model: m, Registry: reg, Store: store, Scenarios: scs})
	if dec.Probe == nil {
		t.Fatalf("want probe; got %+v", dec)
	}
	// Shortest is "short" → its only step is web → probe targets web.
	if dec.Probe.Component != "web" {
		t.Errorf("want probe on web (shortest scenario's terminal); got %q", dec.Probe.Component)
	}
}

func TestOccam_StuckWhenAllContradicted(t *testing.T) {
	m, reg := threeCompModel(t)
	store := facts.NewInMemory()
	// Record a fact that contradicts 'web.down' (status=up, not down).
	store.Append("web", facts.Fact{Key: "status", Value: "up", At: time.Now()})

	scs := []scenarios.Scenario{
		{ID: "a", Chain: []scenarios.Step{{Component: "web", State: "down"}}},
		{ID: "b", Chain: []scenarios.Step{
			{Component: "web", State: "down"},
			{Component: "api", State: "down"},
		}},
	}

	dec := Occam().SuggestProbe(Input{Model: m, Registry: reg, Store: store, Scenarios: scs})
	if !dec.Stuck {
		t.Fatalf("want Stuck=true; got %+v", dec)
	}
}

func TestOccam_DoneWhenSingleRemains(t *testing.T) {
	m, reg := threeCompModel(t)
	store := facts.NewInMemory()
	scs := []scenarios.Scenario{
		{ID: "only", Chain: []scenarios.Step{{Component: "web", State: "down"}}},
	}

	dec := Occam().SuggestProbe(Input{Model: m, Registry: reg, Store: store, Scenarios: scs})
	if !dec.Done {
		t.Fatalf("want Done=true; got %+v", dec)
	}
	if dec.RootCause == nil || dec.RootCause.ID != "only" {
		t.Errorf("want RootCause=only; got %+v", dec.RootCause)
	}
}

func TestOccam_SuspectBumpsTieBreak(t *testing.T) {
	m, reg := threeCompModel(t)
	store := facts.NewInMemory()
	// Two scenarios of equal length (2).
	scs := []scenarios.Scenario{
		{
			ID: "ignored",
			Chain: []scenarios.Step{
				{Component: "web", State: "down"},
				{Component: "api", State: "down"},
			},
		},
		{
			ID: "suspected",
			Chain: []scenarios.Step{
				{Component: "web", State: "down"},
				{Component: "db", State: "down"},
			},
		},
	}
	hints := []SuspectHint{{Component: "db"}}

	dec := Occam().SuggestProbe(Input{
		Model: m, Registry: reg, Store: store,
		Scenarios: scs, Suspects: hints,
	})
	if dec.Probe == nil {
		t.Fatalf("want probe; got %+v", dec)
	}
	// Suspected scenario's terminal is db → probe should target db.
	if dec.Probe.Component != "db" {
		t.Errorf("want probe on db (suspect-touching scenario's terminal); got %q", dec.Probe.Component)
	}
}
