package strategy

import (
	"sort"

	"github.com/mgt-tool/mgtt/internal/facts"
	"github.com/mgt-tool/mgtt/internal/model"
	"github.com/mgt-tool/mgtt/internal/providersupport"
	"github.com/mgt-tool/mgtt/internal/scenarios"
)

type occamStrategy struct{}

// Occam returns the shortest-scenario-first strategy.
func Occam() Strategy { return occamStrategy{} }

func (occamStrategy) Name() string { return "occam" }

func (occamStrategy) SuggestProbe(in Input) Decision {
	live := FilterLive(in.Scenarios, in.Store, in.Model, in.Registry)

	switch len(live) {
	case 0:
		return Decision{Stuck: true, Reason: "no scenario matches observed facts"}
	case 1:
		return Decision{Done: true, RootCause: &live[0], Reason: "single scenario remains"}
	}

	sort.SliceStable(live, func(i, j int) bool {
		if live[i].Length() != live[j].Length() {
			return live[i].Length() < live[j].Length()
		}
		si := touchesAnySuspect(live[i], in.Suspects)
		sj := touchesAnySuspect(live[j], in.Suspects)
		if si != sj {
			return si // true comes first
		}
		ei := crossEliminationCount(live[i], live, in.Store, in.Model, in.Registry)
		ej := crossEliminationCount(live[j], live, in.Store, in.Model, in.Registry)
		if ei != ej {
			return ei > ej
		}
		return live[i].ID < live[j].ID
	})

	chosen := live[0]
	probe := pickSymptomInward(chosen, in.Store, in.Model, in.Registry)
	if probe == nil {
		return Decision{Stuck: true, Reason: "chosen scenario fully verified but live set has multiple"}
	}
	for _, s := range live {
		for _, step := range s.Chain {
			if step.Component == probe.Component {
				probe.Eliminates = append(probe.Eliminates, s.ID)
				break
			}
		}
	}
	return Decision{Probe: probe}
}

func touchesAnySuspect(s scenarios.Scenario, hints []SuspectHint) bool {
	for _, h := range hints {
		if s.TouchesComponent(h.Component) {
			if h.State == "" {
				return true
			}
			for _, step := range s.Chain {
				if step.Component == h.Component && step.State == h.State {
					return true
				}
			}
		}
	}
	return false
}

func crossEliminationCount(s scenarios.Scenario, live []scenarios.Scenario, store *facts.Store, m *model.Model, reg *providersupport.Registry) int {
	probe := pickSymptomInward(s, store, m, reg)
	if probe == nil {
		return 0
	}
	n := 0
	for _, other := range live {
		if other.ID == s.ID {
			continue
		}
		for _, step := range other.Chain {
			if step.Component == probe.Component {
				n++
				break
			}
		}
	}
	return n
}

// pickSymptomInward walks the chain terminal→root and returns a probe
// for the first step whose component has no facts collected. For
// terminal steps, the probe targets the first fact in Observes; for
// non-terminal steps, the first fact (alphabetical) in the type.
func pickSymptomInward(s scenarios.Scenario, store *facts.Store, m *model.Model, reg *providersupport.Registry) *Probe {
	if m == nil || reg == nil {
		return nil
	}
	for i := len(s.Chain) - 1; i >= 0; i-- {
		step := s.Chain[i]
		if store != nil && store.FactsFor(step.Component) != nil {
			continue
		}
		comp := m.Components[step.Component]
		if comp == nil {
			continue
		}
		providers := comp.Providers
		if len(providers) == 0 {
			providers = m.Meta.Providers
		}
		t, providerName, err := reg.ResolveType(providers, comp.Type)
		if err != nil || t == nil {
			continue
		}
		var factName string
		if len(step.Observes) > 0 {
			factName = step.Observes[0]
		} else {
			names := make([]string, 0, len(t.Facts))
			for n := range t.Facts {
				names = append(names, n)
			}
			sort.Strings(names)
			if len(names) > 0 {
				factName = names[0]
			}
		}
		if factName == "" {
			continue
		}
		fs := t.Facts[factName]
		if fs == nil {
			continue
		}
		return &Probe{
			Component: step.Component,
			Fact:      factName,
			Provider:  providerName,
			Cost:      fs.Probe.Cost,
			Access:    fs.Probe.Access,
			Command:   fs.Probe.Cmd,
			ParseMode: fs.Probe.Parse,
		}
	}
	return nil
}
