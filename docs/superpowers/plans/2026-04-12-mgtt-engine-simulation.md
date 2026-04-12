# MGTT Engine + Simulation (Phases 3–4) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the expression evaluator, fact store (in-memory), state derivation, constraint engine, and simulation runner — making `mgtt simulate --all` pass all four storefront scenarios. This is the algorithmic core of MGTT.

**Architecture:** `expr` (leaf) → `facts` (leaf) → `state` (uses expr, provider, facts) → `engine` (uses all) → `simulate` (uses engine with in-memory facts). Expression fields that were raw strings in Plan 1 get compiled to `expr.Node` during loading. The engine is pure — no I/O.

**Tech Stack:** Go, gopkg.in/yaml.v3 (for scenario YAML), no new external dependencies.

**Design doc:** `docs/superpowers/specs/2026-04-12-mgtt-mvp-design.md` — §4.3 (expr), §4.4 (state), §4.6 (engine), §5 (algorithm), §7.4 (simulation runner).

**Depends on:** Plan 1 complete. Packages `model`, `provider`, `render`, `cli` exist. Provider YAML loads with `HealthyRaw`/`WhenRaw`/`WhileRaw` as strings.

---

## File Map

| Action | Path | Responsibility |
|--------|------|----------------|
| Create | `internal/expr/types.go` | Node interface, AndNode, OrNode, CmpNode, Ctx, Value, CmpOp |
| Create | `internal/expr/error.go` | UnresolvedError type |
| Create | `internal/expr/parser.go` | Tokenizer + recursive descent parser |
| Create | `internal/expr/eval.go` | Node.Eval implementations |
| Create | `internal/expr/expr_test.go` | Comprehensive tests |
| Create | `internal/facts/types.go` | Store, Fact, StoreMeta types |
| Create | `internal/facts/store.go` | NewInMemory, Append, Latest |
| Create | `internal/facts/facts_test.go` | Tests |
| Create | `internal/state/derive.go` | Derive function |
| Create | `internal/state/state_test.go` | Tests including ordering subtleties |
| Create | `internal/engine/types.go` | PathTree, Path, Probe types |
| Create | `internal/engine/plan.go` | Plan function — 5 stages |
| Create | `internal/engine/engine_test.go` | Tests against all 4 scenarios |
| Modify | `internal/provider/types.go` | Change HealthyRaw→Healthy ([]expr.Node), StateDef.WhenRaw→When (expr.Node) |
| Modify | `internal/provider/load.go` | Compile expressions during load |
| Modify | `internal/model/types.go` | Change HealthyRaw→Healthy, WhileRaw→While |
| Modify | `internal/model/load.go` | Compile expressions during load |
| Create | `internal/simulate/types.go` | Scenario, Expectation, Result |
| Create | `internal/simulate/run.go` | Run function |
| Create | `internal/simulate/load.go` | Scenario YAML loader |
| Create | `internal/simulate/simulate_test.go` | Tests |
| Create | `internal/render/simulate.go` | SimulateResult, SimulateAll output |
| Create | `internal/cli/simulate.go` | `mgtt simulate --scenario` and `--all` |
| Create | `scenarios/rds-unavailable.yaml` | Scenario 1 |
| Create | `scenarios/api-crash-loop.yaml` | Scenario 2 |
| Create | `scenarios/frontend-degraded.yaml` | Scenario 3 |
| Create | `scenarios/all-healthy.yaml` | Scenario 4 |
| Create | `testdata/golden/simulate_all.txt` | Golden file |

---

## Task 1: Facts package — in-memory store

**Files:**
- Create: `internal/facts/types.go`
- Create: `internal/facts/store.go`
- Create: `internal/facts/facts_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/facts/facts_test.go
package facts_test

import (
	"testing"
	"time"

	"mgtt/internal/facts"
)

func TestNewInMemory(t *testing.T) {
	s := facts.NewInMemory()
	if s == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestAppendAndLatest(t *testing.T) {
	s := facts.NewInMemory()
	s.Append("api", facts.Fact{Key: "ready_replicas", Value: 0, Collector: "simulate", At: time.Now()})
	s.Append("api", facts.Fact{Key: "ready_replicas", Value: 3, Collector: "simulate", At: time.Now()})

	f := s.Latest("api", "ready_replicas")
	if f == nil {
		t.Fatal("expected fact")
	}
	if f.Value != 3 {
		t.Fatalf("expected latest value 3, got %v", f.Value)
	}
}

func TestLatest_NotFound(t *testing.T) {
	s := facts.NewInMemory()
	f := s.Latest("api", "nonexistent")
	if f != nil {
		t.Fatal("expected nil for missing fact")
	}
}

func TestAppend_MultipleFacts(t *testing.T) {
	s := facts.NewInMemory()
	s.Append("api", facts.Fact{Key: "ready_replicas", Value: 0, Collector: "simulate", At: time.Now()})
	s.Append("api", facts.Fact{Key: "restart_count", Value: 12, Collector: "simulate", At: time.Now()})
	s.Append("rds", facts.Fact{Key: "available", Value: false, Collector: "simulate", At: time.Now()})

	if s.Latest("api", "ready_replicas") == nil {
		t.Fatal("missing api.ready_replicas")
	}
	if s.Latest("api", "restart_count") == nil {
		t.Fatal("missing api.restart_count")
	}
	if s.Latest("rds", "available") == nil {
		t.Fatal("missing rds.available")
	}
	if s.Latest("rds", "ready_replicas") != nil {
		t.Fatal("unexpected rds.ready_replicas")
	}
}

func TestFactsForComponent(t *testing.T) {
	s := facts.NewInMemory()
	s.Append("api", facts.Fact{Key: "ready_replicas", Value: 0, Collector: "simulate", At: time.Now()})
	s.Append("api", facts.Fact{Key: "restart_count", Value: 12, Collector: "simulate", At: time.Now()})

	all := s.FactsFor("api")
	if len(all) != 2 {
		t.Fatalf("expected 2 facts, got %d", len(all))
	}
}
```

- [ ] **Step 2: Implement facts package**

```go
// internal/facts/types.go
package facts

import "time"

type Store struct {
	facts map[string][]Fact
}

type StoreMeta struct {
	Model    string
	Version  string
	Incident string
	Started  time.Time
}

type Fact struct {
	Key       string
	Value     any
	Collector string
	At        time.Time
	Note      string
	Raw       string
}
```

```go
// internal/facts/store.go
package facts

func NewInMemory() *Store {
	return &Store{facts: make(map[string][]Fact)}
}

func (s *Store) Append(component string, f Fact) {
	s.facts[component] = append(s.facts[component], f)
}

func (s *Store) Latest(component, key string) *Fact {
	ff := s.facts[component]
	for i := len(ff) - 1; i >= 0; i-- {
		if ff[i].Key == key {
			return &ff[i]
		}
	}
	return nil
}

func (s *Store) FactsFor(component string) []Fact {
	return s.facts[component]
}

func (s *Store) AllComponents() []string {
	var components []string
	seen := make(map[string]bool)
	for c := range s.facts {
		if !seen[c] {
			components = append(components, c)
			seen[c] = true
		}
	}
	return components
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/facts/ -v
```

Expected: all PASS

- [ ] **Step 4: Commit**

```bash
git add internal/facts/
git commit -m "feat(facts): in-memory fact store with Append, Latest, FactsFor"
```

---

## Task 2: Expression parser and evaluator

**Files:**
- Create: `internal/expr/types.go`
- Create: `internal/expr/error.go`
- Create: `internal/expr/parser.go`
- Create: `internal/expr/eval.go`
- Create: `internal/expr/expr_test.go`

This is the most algorithmically complex task. The grammar:

```
expr    = or
or      = and ("|" and)*
and     = primary ("&" primary)*
primary = "(" expr ")" | ref cmp value
ref     = ident ("." ident)?
cmp     = "==" | "!=" | "<" | ">" | "<=" | ">="
value   = int | float | bool | string
```

Precedence: `|` loosest, `&` tighter, comparison tightest (C style).

- [ ] **Step 1: Write failing tests**

```go
// internal/expr/expr_test.go
package expr_test

import (
	"errors"
	"testing"

	"mgtt/internal/expr"
	"mgtt/internal/facts"
)

func makeCtx(component string, factsMap map[string]map[string]any, states map[string]string) expr.Ctx {
	store := facts.NewInMemory()
	for c, kvs := range factsMap {
		for k, v := range kvs {
			store.Append(c, facts.Fact{Key: k, Value: v})
		}
	}
	return expr.Ctx{
		CurrentComponent: component,
		Facts:            store,
		States:           states,
	}
}

func TestParse_SimpleComparison(t *testing.T) {
	node, err := expr.Parse("ready_replicas == 3")
	if err != nil {
		t.Fatal(err)
	}
	if node == nil {
		t.Fatal("expected non-nil node")
	}
}

func TestParse_And(t *testing.T) {
	node, err := expr.Parse("ready_replicas < desired_replicas & restart_count > 5")
	if err != nil {
		t.Fatal(err)
	}
	if node == nil {
		t.Fatal("expected non-nil node")
	}
}

func TestParse_Or(t *testing.T) {
	node, err := expr.Parse("a == 1 | b == 2")
	if err != nil {
		t.Fatal(err)
	}
	if node == nil {
		t.Fatal("expected non-nil node")
	}
}

func TestParse_ComponentDotFact(t *testing.T) {
	node, err := expr.Parse("api.ready_replicas == 0")
	if err != nil {
		t.Fatal(err)
	}
	if node == nil {
		t.Fatal("expected non-nil node")
	}
}

func TestParse_StateComparison(t *testing.T) {
	node, err := expr.Parse("vault.state == starting")
	if err != nil {
		t.Fatal(err)
	}
	if node == nil {
		t.Fatal("expected non-nil node")
	}
}

func TestParse_Bool(t *testing.T) {
	node, err := expr.Parse("available == true")
	if err != nil {
		t.Fatal(err)
	}
	if node == nil {
		t.Fatal("expected non-nil node")
	}
}

func TestParse_Parentheses(t *testing.T) {
	_, err := expr.Parse("(a == 1 | b == 2) & c == 3")
	if err != nil {
		t.Fatal(err)
	}
}

func TestEval_SimpleTrue(t *testing.T) {
	node, _ := expr.Parse("ready_replicas == 3")
	ctx := makeCtx("api", map[string]map[string]any{
		"api": {"ready_replicas": 3},
	}, nil)
	ok, err := node.Eval(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected true")
	}
}

func TestEval_SimpleFalse(t *testing.T) {
	node, _ := expr.Parse("ready_replicas == 3")
	ctx := makeCtx("api", map[string]map[string]any{
		"api": {"ready_replicas": 0},
	}, nil)
	ok, err := node.Eval(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected false")
	}
}

func TestEval_Unresolved(t *testing.T) {
	node, _ := expr.Parse("restart_count > 5")
	ctx := makeCtx("api", map[string]map[string]any{}, nil)
	ok, err := node.Eval(ctx)
	if ok {
		t.Fatal("expected false for unresolved")
	}
	var ue *expr.UnresolvedError
	if !errors.As(err, &ue) {
		t.Fatalf("expected UnresolvedError, got %v", err)
	}
	if ue.Fact != "restart_count" {
		t.Fatalf("expected fact 'restart_count', got %q", ue.Fact)
	}
}

func TestEval_And_BothTrue(t *testing.T) {
	node, _ := expr.Parse("ready_replicas < desired_replicas & restart_count > 5")
	ctx := makeCtx("api", map[string]map[string]any{
		"api": {"ready_replicas": 0, "desired_replicas": 3, "restart_count": 12},
	}, nil)
	ok, err := node.Eval(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected true")
	}
}

func TestEval_And_OneUnresolved(t *testing.T) {
	node, _ := expr.Parse("ready_replicas < desired_replicas & restart_count > 5")
	ctx := makeCtx("api", map[string]map[string]any{
		"api": {"ready_replicas": 0, "desired_replicas": 3},
	}, nil)
	ok, err := node.Eval(ctx)
	if ok {
		t.Fatal("expected false when one operand unresolved")
	}
	var ue *expr.UnresolvedError
	if !errors.As(err, &ue) {
		t.Fatalf("expected UnresolvedError, got %v", err)
	}
}

func TestEval_StateComparison(t *testing.T) {
	node, _ := expr.Parse("vault.state == starting")
	ctx := makeCtx("api", nil, map[string]string{"vault": "starting"})
	ok, err := node.Eval(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected true")
	}
}

func TestEval_BoolFact(t *testing.T) {
	node, _ := expr.Parse("available == true")
	ctx := makeCtx("rds", map[string]map[string]any{
		"rds": {"available": true},
	}, nil)
	ok, err := node.Eval(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected true")
	}
}

func TestEval_CrossComponent(t *testing.T) {
	node, _ := expr.Parse("api.ready_replicas == 0")
	ctx := makeCtx("nginx", map[string]map[string]any{
		"api": {"ready_replicas": 0},
	}, nil)
	ok, err := node.Eval(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected true")
	}
}

// Kubernetes deployment state: degraded = ready_replicas < desired_replicas & restart_count > 5
func TestEval_K8sDegraded(t *testing.T) {
	node, _ := expr.Parse("ready_replicas < desired_replicas & restart_count > 5")
	ctx := makeCtx("api", map[string]map[string]any{
		"api": {"ready_replicas": 0, "desired_replicas": 3, "restart_count": 47},
	}, nil)
	ok, err := node.Eval(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected true for degraded state")
	}
}

// Starting: ready_replicas < desired_replicas (no restart_count check)
func TestEval_K8sStarting(t *testing.T) {
	node, _ := expr.Parse("ready_replicas < desired_replicas")
	ctx := makeCtx("api", map[string]map[string]any{
		"api": {"ready_replicas": 0, "desired_replicas": 3},
	}, nil)
	ok, err := node.Eval(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected true for starting state")
	}
}
```

- [ ] **Step 2: Implement expr package**

**types.go:**
```go
package expr

import "mgtt/internal/facts"

type CmpOp int
const (
	OpEq CmpOp = iota
	OpNeq
	OpLt
	OpGt
	OpLte
	OpGte
)

type Value struct {
	IntVal    *int
	FloatVal  *float64
	BoolVal   *bool
	StringVal *string
}

type Node interface {
	Eval(ctx Ctx) (bool, error)
}

type AndNode struct{ L, R Node }
type OrNode  struct{ L, R Node }
type CmpNode struct {
	Component string // empty = use CurrentComponent
	Fact      string // or "state"
	Op        CmpOp
	Value     Value
}

type Ctx struct {
	CurrentComponent string
	Facts            *facts.Store
	States           map[string]string // derived component states
}
```

**error.go:**
```go
package expr

import "fmt"

type UnresolvedError struct {
	Component string
	Fact      string
	Reason    string // "missing", "stale", "type mismatch"
}

func (e *UnresolvedError) Error() string {
	return fmt.Sprintf("unresolved: %s.%s (%s)", e.Component, e.Fact, e.Reason)
}
```

**parser.go** — Hand-rolled recursive descent. Tokenizer splits on whitespace and operators. Parser follows the grammar with `|` at the top level, `&` inside, comparison at the leaf.

Key parsing considerations:
- `ready_replicas == 3` → CmpNode{Fact: "ready_replicas", Op: OpEq, Value: {IntVal: 3}}
- `api.ready_replicas == 0` → CmpNode{Component: "api", Fact: "ready_replicas", ...}
- `vault.state == starting` → CmpNode{Component: "vault", Fact: "state", Value: {StringVal: "starting"}}
- `available == true` → CmpNode{Fact: "available", Value: {BoolVal: true}}
- Numeric values: try int first, then float
- Bool values: `true`/`false`
- Everything else: string (unquoted identifiers like state names)

**eval.go** — Eval implementations for each Node type:

`CmpNode.Eval`:
1. Resolve the fact: if Fact == "state", look up `ctx.States[component]` and compare to Value as string. If state not found, return UnresolvedError.
2. Otherwise: look up `ctx.Facts.Latest(component, fact)`. If nil, return UnresolvedError{Reason: "missing"}.
3. Compare fact.Value to node.Value using the operator. Handle numeric type coercion (int→float promotion for mixed comparisons).

`AndNode.Eval`: evaluate L, then R. If either returns UnresolvedError, propagate it (AND requires both sides). If L is false (not unresolved), short-circuit to false.

`OrNode.Eval`: evaluate L, then R. If L is true, short-circuit to true. If both unresolved, propagate. If one is true and other unresolved, return true.

- [ ] **Step 3: Run tests**

```bash
go test ./internal/expr/ -v
```

Expected: all 15+ tests PASS

- [ ] **Step 4: Commit**

```bash
git add internal/expr/
git commit -m "feat(expr): expression parser and evaluator with UnresolvedError semantics"
```

---

## Task 3: Compile raw expressions in provider and model loading

**Files:**
- Modify: `internal/provider/types.go` — change `HealthyRaw []string` to `Healthy []Node`, `WhenRaw string` to `When expr.Node` in StateDef
- Modify: `internal/provider/load.go` — compile expressions during loading
- Modify: `internal/model/types.go` — change `HealthyRaw []string` to `HealthyRaw []string` (keep raw for model, compile in engine/validate) OR add compiled fields
- Modify: `internal/model/load.go` — compile `WhileRaw` to `While expr.Node`
- Update all tests and render code that references `HealthyRaw`/`WhenRaw`

**Important design decision:** Provider types get compiled fields (`Healthy []expr.Node`, `StateDef.When expr.Node`). Model components keep BOTH raw and compiled forms: `HealthyRaw []string` stays for the render layer, `Healthy []expr.Node` added for engine use. `Dependency.WhileRaw` stays, `Dependency.While expr.Node` added.

For StateDef in provider:
```go
type StateDef struct {
    Name        string
    WhenRaw     string    // keep for display
    When        expr.Node // compiled — used by state.Derive
    Description string
}
```

For Component in model:
```go
type Component struct {
    Name         string
    Type         string
    Providers    []string
    Depends      []Dependency
    HealthyRaw   []string            // keep for render
    FailureModes map[string][]string
}

type Dependency struct {
    On       []string
    WhileRaw string    // keep for render
    While    expr.Node // compiled — nil means always active
}
```

In `provider/load.go`, after parsing a StateDef, compile `WhenRaw`:
```go
sd.When, err = expr.Parse(sd.WhenRaw)
```

In `provider/load.go`, compile `Healthy` conditions:
```go
for _, raw := range rt.Healthy {
    node, err := expr.Parse(raw)
    typ.Healthy = append(typ.Healthy, node)
}
```

In `model/load.go`, compile `Dependency.While`:
```go
if rd.While != "" {
    dep.While, err = expr.Parse(dep.WhileRaw)
}
```

Update existing tests to pass (render functions that used `HealthyRaw` should still work since we keep the raw field).

- [ ] **Step 1: Update provider types and loader**
- [ ] **Step 2: Update model types and loader**
- [ ] **Step 3: Fix all broken tests (render, model_test, provider_test, cli_test)**
- [ ] **Step 4: Run full test suite**

```bash
go vet ./... && go test ./...
```

- [ ] **Step 5: Commit**

```bash
git add internal/provider/ internal/model/ internal/render/ internal/cli/
git commit -m "feat: compile expression strings to expr.Node during provider and model loading"
```

---

## Task 4: State derivation

**Files:**
- Create: `internal/state/derive.go`
- Create: `internal/state/state_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/state/state_test.go
package state_test

import (
	"errors"
	"testing"
	"time"

	"mgtt/internal/expr"
	"mgtt/internal/facts"
	"mgtt/internal/model"
	"mgtt/internal/provider"
	"mgtt/internal/state"
)

func loadStorefront(t *testing.T) (*model.Model, *provider.Registry) {
	t.Helper()
	m, err := model.Load("../../examples/storefront/system.model.yaml")
	if err != nil {
		t.Fatal(err)
	}
	k8s, err := provider.LoadFromFile("../../providers/kubernetes/provider.yaml")
	if err != nil {
		t.Fatal(err)
	}
	aws, err := provider.LoadFromFile("../../providers/aws/provider.yaml")
	if err != nil {
		t.Fatal(err)
	}
	reg := provider.NewRegistry()
	reg.Register(k8s)
	reg.Register(aws)
	return m, reg
}

func injectFacts(facts map[string]map[string]any) *facts.Store {
	s := facts.NewInMemory()
	for c, kvs := range facts {
		for k, v := range kvs {
			s.Append(c, facts.Fact{Key: k, Value: v, Collector: "test", At: time.Now()})
		}
	}
	return s
}

// api crash-looping: degraded
func TestDerive_APIDegraded(t *testing.T) {
	m, reg := loadStorefront(t)
	store := injectFacts(map[string]map[string]any{
		"api": {"ready_replicas": 0, "desired_replicas": 3, "restart_count": 47},
	})
	d := state.Derive(m, reg, store)
	if d.ComponentStates["api"] != "degraded" {
		t.Fatalf("expected 'degraded', got %q", d.ComponentStates["api"])
	}
}

// api starting (no restart_count): starting, NOT degraded
func TestDerive_APIStarting_MissingRestartCount(t *testing.T) {
	m, reg := loadStorefront(t)
	store := injectFacts(map[string]map[string]any{
		"api": {"ready_replicas": 0, "desired_replicas": 3},
	})
	d := state.Derive(m, reg, store)
	if d.ComponentStates["api"] != "starting" {
		t.Fatalf("expected 'starting', got %q", d.ComponentStates["api"])
	}
	// Check that degraded was unresolved
	if len(d.UnresolvedBy["api"]) == 0 {
		t.Fatal("expected unresolved entries for api")
	}
}

// api live: ready == desired
func TestDerive_APILive(t *testing.T) {
	m, reg := loadStorefront(t)
	store := injectFacts(map[string]map[string]any{
		"api": {"ready_replicas": 3, "desired_replicas": 3},
	})
	d := state.Derive(m, reg, store)
	if d.ComponentStates["api"] != "live" {
		t.Fatalf("expected 'live', got %q", d.ComponentStates["api"])
	}
}

// rds live
func TestDerive_RDSLive(t *testing.T) {
	m, reg := loadStorefront(t)
	store := injectFacts(map[string]map[string]any{
		"rds": {"available": true},
	})
	d := state.Derive(m, reg, store)
	if d.ComponentStates["rds"] != "live" {
		t.Fatalf("expected 'live', got %q", d.ComponentStates["rds"])
	}
}

// rds stopped
func TestDerive_RDSStopped(t *testing.T) {
	m, reg := loadStorefront(t)
	store := injectFacts(map[string]map[string]any{
		"rds": {"available": false},
	})
	d := state.Derive(m, reg, store)
	if d.ComponentStates["rds"] != "stopped" {
		t.Fatalf("expected 'stopped', got %q", d.ComponentStates["rds"])
	}
}

// no facts at all: everything unknown
func TestDerive_NoFacts(t *testing.T) {
	m, reg := loadStorefront(t)
	store := facts.NewInMemory()
	d := state.Derive(m, reg, store)
	for _, name := range m.Order {
		if d.ComponentStates[name] != "unknown" {
			t.Errorf("%s: expected 'unknown', got %q", name, d.ComponentStates[name])
		}
	}
}
```

NOTE: There may be a naming conflict with `facts` as both the import and the variable name. Use an import alias if needed: `factspkg "mgtt/internal/facts"`.

- [ ] **Step 2: Implement state derivation**

```go
// internal/state/derive.go
package state

import (
	"errors"

	"mgtt/internal/expr"
	"mgtt/internal/facts"
	"mgtt/internal/model"
	"mgtt/internal/provider"
)

type Derivation struct {
	ComponentStates map[string]string
	UnresolvedBy    map[string][]expr.UnresolvedError
}

func Derive(m *model.Model, reg *provider.Registry, store *facts.Store) *Derivation {
	d := &Derivation{
		ComponentStates: make(map[string]string),
		UnresolvedBy:    make(map[string][]expr.UnresolvedError),
	}

	for _, name := range m.Order {
		comp := m.Components[name]
		providers := comp.Providers
		if providers == nil {
			providers = m.Meta.Providers
		}

		typ, provName, err := reg.ResolveType(providers, comp.Type)
		if err != nil {
			d.ComponentStates[name] = "unknown"
			continue
		}

		states, _ := reg.StatesFor(provName, typ.Name)
		matched := false
		ctx := expr.Ctx{
			CurrentComponent: name,
			Facts:            store,
			States:           d.ComponentStates, // partial — earlier components already resolved
		}

		for _, sd := range states {
			if sd.When == nil {
				continue
			}
			ok, evalErr := sd.When.Eval(ctx)
			if evalErr != nil {
				var ue *expr.UnresolvedError
				if errors.As(evalErr, &ue) {
					d.UnresolvedBy[name] = append(d.UnresolvedBy[name], *ue)
					continue // try next state
				}
				continue // other error — skip
			}
			if ok {
				d.ComponentStates[name] = sd.Name
				matched = true
				break
			}
		}
		if !matched {
			d.ComponentStates[name] = "unknown"
		}
	}

	return d
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/state/ -v
```

Expected: all PASS

- [ ] **Step 4: Commit**

```bash
git add internal/state/
git commit -m "feat(state): state derivation with first-match-wins and UnresolvedError tracking"
```

---

## Task 5: Constraint engine — Plan function

**Files:**
- Create: `internal/engine/types.go`
- Create: `internal/engine/plan.go`
- Create: `internal/engine/engine_test.go`

This is the core of MGTT. The engine implements 5 stages (design doc §5.2) and is tested against all 4 scenario expectations (design doc §5.4).

- [ ] **Step 1: Write failing tests — the 4 scenario expectations**

```go
// internal/engine/engine_test.go
package engine_test

import (
	"testing"
	"time"

	"mgtt/internal/engine"
	"mgtt/internal/facts"
	"mgtt/internal/model"
	"mgtt/internal/provider"
)

func setup(t *testing.T) (*model.Model, *provider.Registry) {
	t.Helper()
	m, err := model.Load("../../examples/storefront/system.model.yaml")
	if err != nil {
		t.Fatal(err)
	}
	k8s, _ := provider.LoadFromFile("../../providers/kubernetes/provider.yaml")
	aws, _ := provider.LoadFromFile("../../providers/aws/provider.yaml")
	reg := provider.NewRegistry()
	reg.Register(k8s)
	reg.Register(aws)
	return m, reg
}

func inject(kvs map[string]map[string]any) *facts.Store {
	s := facts.NewInMemory()
	for c, fs := range kvs {
		for k, v := range fs {
			s.Append(c, facts.Fact{Key: k, Value: v, Collector: "test", At: time.Now()})
		}
	}
	return s
}

// Scenario 1: rds unavailable → root_cause=rds, path=[nginx,api,rds], eliminated=[frontend]
func TestPlan_RDSUnavailable(t *testing.T) {
	m, reg := setup(t)
	store := inject(map[string]map[string]any{
		"rds": {"available": false, "connection_count": 0},
		"api": {"ready_replicas": 0, "restart_count": 12, "desired_replicas": 3},
	})
	tree := engine.Plan(m, reg, store, "")
	if tree.RootCause != "rds" {
		t.Fatalf("expected root_cause='rds', got %q", tree.RootCause)
	}
	assertPathExists(t, tree.Paths, []string{"nginx", "api", "rds"})
	assertEliminated(t, tree.Eliminated, "frontend")
}

// Scenario 2: api crash-loop, rds healthy → root_cause=api, eliminated=[rds, frontend]
func TestPlan_APICrashLoop(t *testing.T) {
	m, reg := setup(t)
	store := inject(map[string]map[string]any{
		"api": {"ready_replicas": 0, "restart_count": 24, "desired_replicas": 3},
		"rds": {"available": true, "connection_count": 120},
	})
	tree := engine.Plan(m, reg, store, "")
	if tree.RootCause != "api" {
		t.Fatalf("expected root_cause='api', got %q", tree.RootCause)
	}
	assertEliminated(t, tree.Eliminated, "rds")
	assertEliminated(t, tree.Eliminated, "frontend")
}

// Scenario 3: frontend degraded, api/rds healthy → root_cause=frontend, eliminated=[api, rds]
func TestPlan_FrontendDegraded(t *testing.T) {
	m, reg := setup(t)
	store := inject(map[string]map[string]any{
		"frontend": {"ready_replicas": 0, "restart_count": 8, "desired_replicas": 2},
		"api":      {"ready_replicas": 3, "desired_replicas": 3, "endpoints": 3},
		"rds":      {"available": true, "connection_count": 98},
	})
	tree := engine.Plan(m, reg, store, "")
	if tree.RootCause != "frontend" {
		t.Fatalf("expected root_cause='frontend', got %q", tree.RootCause)
	}
	assertEliminated(t, tree.Eliminated, "api")
	assertEliminated(t, tree.Eliminated, "rds")
}

// Scenario 4: all healthy → root_cause=none, all eliminated
func TestPlan_AllHealthy(t *testing.T) {
	m, reg := setup(t)
	store := inject(map[string]map[string]any{
		"nginx":    {"upstream_count": 4},
		"frontend": {"ready_replicas": 2, "desired_replicas": 2, "endpoints": 2},
		"api":      {"ready_replicas": 3, "desired_replicas": 3, "endpoints": 3},
		"rds":      {"available": true, "connection_count": 87},
	})
	tree := engine.Plan(m, reg, store, "")
	if tree.RootCause != "" {
		t.Fatalf("expected no root cause, got %q", tree.RootCause)
	}
	if len(tree.Paths) != 0 {
		t.Fatalf("expected 0 live paths, got %d", len(tree.Paths))
	}
}

// Entry point detection
func TestPlan_EntryPointIsNginx(t *testing.T) {
	m, reg := setup(t)
	store := facts.NewInMemory()
	tree := engine.Plan(m, reg, store, "")
	if tree.Entry != "nginx" {
		t.Fatalf("expected entry 'nginx', got %q", tree.Entry)
	}
}

// Explicit entry override
func TestPlan_ExplicitEntry(t *testing.T) {
	m, reg := setup(t)
	store := inject(map[string]map[string]any{
		"api": {"ready_replicas": 0, "restart_count": 24, "desired_replicas": 3},
		"rds": {"available": true, "connection_count": 120},
	})
	tree := engine.Plan(m, reg, store, "api")
	if tree.Entry != "api" {
		t.Fatalf("expected entry 'api', got %q", tree.Entry)
	}
}

func assertPathExists(t *testing.T, paths []engine.Path, expected []string) {
	t.Helper()
	for _, p := range paths {
		if sliceEqual(p.Components, expected) {
			return
		}
	}
	t.Fatalf("expected path %v not found in %v", expected, paths)
}

func assertEliminated(t *testing.T, eliminated []engine.Path, component string) {
	t.Helper()
	for _, p := range eliminated {
		for _, c := range p.Components {
			if c == component {
				return
			}
		}
	}
	t.Fatalf("expected %q to be eliminated", component)
}

func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
```

- [ ] **Step 2: Implement engine**

**types.go:**
```go
package engine

import "mgtt/internal/state"

type PathTree struct {
	Entry      string
	Paths      []Path
	Eliminated []Path
	Suggested  *Probe
	RootCause  string
	States     *state.Derivation
}

type Path struct {
	ID         string
	Components []string
	Hypothesis string
	Reason     string
}

type Probe struct {
	Component  string
	Fact       string
	Eliminates []string
	Cost       string
	Access     string
	Command    string
}
```

**plan.go:** Implement the 5 stages from design doc §5.2:

```go
func Plan(m *model.Model, reg *provider.Registry, store *facts.Store, entry string) *PathTree
```

Stage 1: Entry selection — use model.EntryPoint() if entry is empty.
Stage 2: Path enumeration — DFS from entry through all active dependency edges. Each maximal walk is a path. Assign IDs (PATH A, PATH B, ...).
Stage 3: Elimination — for each path, check if components are known-healthy (state matches default_active_state). Eliminate if so.
Stage 4: Failure-mode filtering — for surviving paths, check can_cause chains (soft ranking, not elimination).
Stage 5: Probe ranking — score = paths_eliminated_if_healthy / cost_weight. Tiebreak by distance from entry.

Terminal condition: one remaining path with deepest component in non-default state → set RootCause.
All paths eliminated: RootCause="" (no root cause, everything healthy).

- [ ] **Step 3: Run tests**

```bash
go test ./internal/engine/ -v
```

Expected: all 6 tests PASS (the 4 scenario tests are the critical ones)

- [ ] **Step 4: Commit**

```bash
git add internal/engine/
git commit -m "feat(engine): constraint engine with 5-stage Plan — all 4 scenario expectations pass"
```

---

## Task 6: Simulation runner + scenarios + CLI

**Files:**
- Create: `internal/simulate/types.go`
- Create: `internal/simulate/load.go`
- Create: `internal/simulate/run.go`
- Create: `internal/simulate/simulate_test.go`
- Create: `internal/render/simulate.go`
- Create: `internal/cli/simulate.go`
- Create: `scenarios/rds-unavailable.yaml`
- Create: `scenarios/api-crash-loop.yaml`
- Create: `scenarios/frontend-degraded.yaml`
- Create: `scenarios/all-healthy.yaml`
- Create: `testdata/golden/simulate_all.txt`

### Scenario YAML format:

```yaml
name: rds unavailable
description: >
  rds stops accepting connections.
  api starts crash-looping as a result.
  engine should trace the fault to rds, not api.

inject:
  rds:
    available: false
    connection_count: 0
  api:
    ready_replicas: 0
    restart_count: 12
    desired_replicas: 3

expect:
  root_cause: rds
  path: [nginx, api, rds]
  eliminated: [frontend]
```

### Simulation types:

```go
type Scenario struct {
	Name        string
	Description string
	Inject      map[string]map[string]any
	Expect      Expectation
}

type Expectation struct {
	RootCause  string
	Path       []string
	Eliminated []string
}

type Result struct {
	Scenario *Scenario
	Actual   Expectation
	Pass     bool
	Tree     *engine.PathTree
}
```

### Run function:

```go
func Run(m *model.Model, reg *provider.Registry, sc *Scenario) *Result {
	store := facts.NewInMemory()
	for c, kvs := range sc.Inject {
		for k, v := range kvs {
			store.Append(c, facts.Fact{Key: k, Value: v, Collector: "simulate", At: time.Now()})
		}
	}
	tree := engine.Plan(m, reg, store, "")
	actual := extractConclusion(tree)
	return &Result{
		Scenario: sc,
		Actual:   actual,
		Pass:     matches(sc.Expect, actual),
		Tree:     tree,
	}
}
```

### CLI:

```
mgtt simulate --scenario <file>    run one scenario
mgtt simulate --all                run all files in scenarios/
```

`--all` scans `scenarios/` directory for `*.yaml` files.

### Render:

```go
func SimulateResult(w io.Writer, result *simulate.Result)
// Shows: scenario name, mode (simulation), injected facts, ✓/✗ for root_cause/path/eliminated, pass/fail

func SimulateAll(w io.Writer, results []simulate.Result)
// Shows: one line per scenario (name, pass/fail), then summary (N/M passed)
```

### Tests:

1. Load all 4 scenario files and verify they parse
2. Run all 4 scenarios against storefront model — all should pass
3. Golden test: `mgtt simulate --all` output captured and compared

- [ ] **Step 1: Create scenario YAML files (exact content from simulation-scenario.md)**
- [ ] **Step 2: Implement simulate package (types, load, run)**
- [ ] **Step 3: Implement render simulate functions**
- [ ] **Step 4: Implement CLI simulate command**
- [ ] **Step 5: Write tests — scenario loading + run + golden**
- [ ] **Step 6: Run full test suite**

```bash
go vet ./... && go test ./...
```

- [ ] **Step 7: Build binary and verify**

```bash
go build ./cmd/mgtt
./mgtt simulate --all
```

Expected output:
```
  rds-unavailable        ✓ passed
  api-crash-loop         ✓ passed
  frontend-degraded      ✓ passed
  all-healthy            ✓ passed

  4/4 scenarios passed
```

- [ ] **Step 8: Capture golden file and verify golden test**

```bash
./mgtt simulate --all > testdata/golden/simulate_all.txt
go test ./internal/cli/ -v -run TestGolden_SimulateAll
```

- [ ] **Step 9: Commit**

```bash
git add internal/simulate/ internal/render/simulate.go internal/cli/simulate.go scenarios/ testdata/golden/simulate_all.txt
git commit -m "feat(simulate): simulation runner with 4 storefront scenarios — all passing"
```

---

## Task 7: Final verification

- [ ] **Step 1: Full test suite**

```bash
go vet ./... && go test ./...
```

Expected: all packages green.

- [ ] **Step 2: End-to-end CLI verification**

```bash
./mgtt model validate examples/storefront/system.model.yaml
./mgtt simulate --all
./mgtt simulate --scenario scenarios/rds-unavailable.yaml
./mgtt provider ls
./mgtt stdlib ls
```

- [ ] **Step 3: Tag milestone**

```bash
git tag v0.0.2-engine
```

---

## Acceptance Criteria (Phases 3–4 complete when)

1. `go vet && go test` all green across all packages
2. `mgtt simulate --all` passes all 4 storefront scenarios
3. `mgtt simulate --scenario scenarios/rds-unavailable.yaml` shows detailed pass output
4. Expression parser handles: `==`, `!=`, `<`, `>`, `<=`, `>=`, `&`, `|`, `()`, bare facts, component.fact, component.state, int/float/bool values
5. `UnresolvedError` propagates correctly (missing facts don't cause false-negative eliminations)
6. State derivation: `degraded` before `starting` ordering works (api with restart_count=47 → degraded, not starting)
7. State derivation: missing `restart_count` → `starting` (not unknown), with unresolved entry for `degraded`
8. Engine scenarios match design doc §5.4:
   - rds-unavailable → root_cause=rds, path=[nginx,api,rds], eliminated=[frontend]
   - api-crash-loop → root_cause=api, eliminated=[rds,frontend]
   - frontend-degraded → root_cause=frontend, eliminated=[api,rds]
   - all-healthy → root_cause=none, all eliminated
9. Golden test for `simulate --all` passes

## What's Next

**Plan 3** (Phases 5–7): Fact store (disk-backed), incident lifecycle, probe execution (exec + fixture backends), interactive `mgtt plan` loop. Troubleshooting modus operandi.
