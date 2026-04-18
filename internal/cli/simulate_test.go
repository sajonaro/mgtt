package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgt-tool/mgtt/internal/scenarios"
	"github.com/mgt-tool/mgtt/internal/simulate"
)

func TestEmitGapWarning_RootInScenarios_NoWarning(t *testing.T) {
	c := &simulate.Scenario{
		Name:   "db-outage",
		Expect: simulate.Expectation{RootCause: "db"},
	}
	enum := []scenarios.Scenario{
		{ID: "s-1", Root: scenarios.RootRef{Component: "db", State: "down"}},
	}
	var buf bytes.Buffer
	emitGapWarning(&buf, c, enum)
	if buf.Len() != 0 {
		t.Errorf("want no warning; got %q", buf.String())
	}
}

func TestEmitGapWarning_RootMissing_Warns(t *testing.T) {
	c := &simulate.Scenario{
		Name:   "rds-outage",
		Expect: simulate.Expectation{RootCause: "rds"},
	}
	enum := []scenarios.Scenario{
		{ID: "s-1", Root: scenarios.RootRef{Component: "db", State: "down"}},
	}
	var buf bytes.Buffer
	emitGapWarning(&buf, c, enum)
	if !strings.Contains(buf.String(), "WARN:") {
		t.Errorf("want WARN; got %q", buf.String())
	}
	if !strings.Contains(buf.String(), "rds") {
		t.Errorf("want warning to mention rds; got %q", buf.String())
	}
}

func TestEmitGapWarning_UnenumeratedIntentional_Suppresses(t *testing.T) {
	c := &simulate.Scenario{
		Name:                    "rds-outage",
		Expect:                  simulate.Expectation{RootCause: "rds"},
		UnenumeratedIntentional: true,
	}
	enum := []scenarios.Scenario{
		{ID: "s-1", Root: scenarios.RootRef{Component: "db", State: "down"}},
	}
	var buf bytes.Buffer
	emitGapWarning(&buf, c, enum)
	if buf.Len() != 0 {
		t.Errorf("want suppressed warning; got %q", buf.String())
	}
}

func TestEmitGapWarning_NoEnumerated_Silent(t *testing.T) {
	c := &simulate.Scenario{
		Name:   "rds-outage",
		Expect: simulate.Expectation{RootCause: "rds"},
	}
	var buf bytes.Buffer
	emitGapWarning(&buf, c, nil)
	if buf.Len() != 0 {
		t.Errorf("want silent when no enumerated scenarios; got %q", buf.String())
	}
}

func TestEmitGapWarning_RootCauseNone_Silent(t *testing.T) {
	c := &simulate.Scenario{
		Name:   "all-healthy",
		Expect: simulate.Expectation{RootCause: "none"},
	}
	enum := []scenarios.Scenario{
		{ID: "s-1", Root: scenarios.RootRef{Component: "db", State: "down"}},
	}
	var buf bytes.Buffer
	emitGapWarning(&buf, c, enum)
	if buf.Len() != 0 {
		t.Errorf("want silent for root=none; got %q", buf.String())
	}
}
