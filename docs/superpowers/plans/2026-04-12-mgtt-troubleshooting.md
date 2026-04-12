# MGTT Troubleshooting (Phases 5–7) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the disk-backed fact store, incident lifecycle, probe execution (exec + fixture backends), and interactive `mgtt plan` loop — making the full troubleshooting walkthrough from `troubleshooting-scenario.md` work end-to-end against the fixture backend.

**Architecture:** Extends the in-memory facts store with disk I/O (`system.state.yaml`). Incident lifecycle manages session files. Probe execution uses the `Executor` interface with `exec` and `fixture` backends selected via `$MGTT_FIXTURES`. Interactive plan loop runs in CLI, auto-accepts in non-TTY mode for golden tests.

**Tech Stack:** Go, gopkg.in/yaml.v3, os/exec, golang.org/x/term (for TTY detection)

**Design doc:** `docs/superpowers/specs/2026-04-12-mgtt-mvp-design.md` — §4.2 (facts), §6.4-6.7 (executor, fixtures, parse, substitution), §7.2 (plan loop), §7.5 (incident).

**Depends on:** Plans 1-2 complete. All 9 packages exist. Engine Plan works. Simulation passes.

---

## File Map

| Action | Path | Responsibility |
|--------|------|----------------|
| Modify | `internal/facts/types.go` | Add StoreMeta, disk I/O fields |
| Create | `internal/facts/disk.go` | NewDiskBacked, Load, Save (state.yaml I/O) |
| Modify | `internal/facts/facts_test.go` | Disk I/O tests |
| Create | `internal/incident/incident.go` | Incident struct, Start, End, Current |
| Create | `internal/incident/incident_test.go` | Tests |
| Create | `internal/probe/types.go` | Executor interface, Command, Result |
| Create | `internal/probe/parse.go` | 8 parse modes |
| Create | `internal/probe/substitute.go` | Command template substitution |
| Create | `internal/probe/fixture/executor.go` | Fixture backend |
| Create | `internal/probe/exec/executor.go` | Real exec backend |
| Create | `internal/probe/probe.go` | NewExecutor (env var dispatch) |
| Create | `internal/probe/probe_test.go` | Parse + fixture tests |
| Create | `internal/render/plan.go` | Plan output, probe results, root cause |
| Create | `internal/render/incident.go` | Incident start/end output |
| Create | `internal/render/facts.go` | Facts list, status output |
| Create | `internal/cli/plan.go` | `mgtt plan` interactive loop |
| Create | `internal/cli/incident.go` | `mgtt incident start/end` |
| Create | `internal/cli/fact_add.go` | `mgtt fact add` |
| Create | `internal/cli/ls.go` | `mgtt ls`, `mgtt ls facts`, `mgtt ls components` |
| Create | `internal/cli/status.go` | `mgtt status` |
| Create | `fixtures/storefront-incident.yaml` | Fixture data for troubleshooting scenario |
| Create | `testdata/golden/plan_storefront.txt` | Golden file for plan loop |

---

## Task 1: Facts disk I/O

Extend the existing facts package with disk-backed store that reads/writes `system.state.yaml`.

**Format (spec §7):**
```yaml
meta:
  model: storefront
  version: "1.0"
  incident: inc-20240205-0814-001
  started: 2024-02-05T08:14:00Z

facts:
  api:
    - key: endpoints
      value: 0
      collector: kubernetes
      at: 2024-02-05T08:15:12Z
    - key: ready_replicas
      value: 0
      collector: kubernetes
      at: 2024-02-05T08:15:18Z
  rds:
    - key: available
      value: true
      collector: aws
      at: 2024-02-05T08:15:31Z
```

**Implementation:**
- `NewDiskBacked(path string) *Store` — wraps the in-memory store, adds a file path
- `Load(path string) (*Store, error)` — reads state.yaml into Store
- `Save() error` — writes Store to disk (atomic rename: write .tmp, rename)
- `AppendAndSave(component string, f Fact) error` — append + save

---

## Task 2: Incident lifecycle

**Implementation:**
- `Start(modelName, modelVersion string, id string) (*Incident, error)` — generates ID if empty, creates state.yaml, writes `.mgtt-current`
- `End(inc *Incident) error` — sets ended time, clears `.mgtt-current`
- `Current() (*Incident, error)` — reads `.mgtt-current`, loads incident
- ID format: `inc-YYYYMMDD-HHMM-NNN`

---

## Task 3: Probe execution — parse modes + fixture backend

**Parse modes (8 from spec §8.5.2):**
- `int` — trim, strconv.Atoi
- `float` — trim, strconv.ParseFloat
- `bool` — true/1/yes → true; false/0/no → false
- `string` — trim
- `exit_code` — exit 0 → true; non-zero → false
- `json:<path>` — parse JSON, dot-path extract
- `lines:<N>` — count non-empty lines
- `regex:<pat>` — first capture group

**Fixture backend:** reads YAML keyed by (provider, component, fact), returns canned stdout + exit code.

**Fixture file format:**
```yaml
kubernetes:
  nginx:
    upstream_count: { stdout: "0\n", exit: 0 }
  api:
    endpoints: { stdout: "\n", exit: 0 }
    ready_replicas: { stdout: "0\n", exit: 0 }
    desired_replicas: { stdout: "3\n", exit: 0 }
    restart_count: { stdout: "47\n", exit: 0 }
  frontend:
    ready_replicas: { stdout: "2\n", exit: 0 }
    desired_replicas: { stdout: "2\n", exit: 0 }
    endpoints: { stdout: "10.0.1.2\n10.0.1.3\n", exit: 0 }
aws:
  rds:
    available: { stdout: "available\n", exit: 0 }
    connection_count: { stdout: "498\n", exit: 0 }
```

**Command substitution:** `probe.Substitute(template, component, modelVars, providerVars)`

---

## Task 4: Render plan + incident + facts output

Render functions for the interactive plan loop and incident lifecycle.

---

## Task 5: CLI commands + interactive plan loop + golden test

**Commands:**
- `mgtt incident start [--id ID]`
- `mgtt incident end`
- `mgtt plan [--component NAME] [--model PATH]`
- `mgtt fact add <component> <key> <value> [--note TEXT]`
- `mgtt ls` / `mgtt ls components` / `mgtt ls facts [component]`
- `mgtt status`

**Interactive plan loop:**
Non-TTY auto-accept for golden tests. Uses `MGTT_FIXTURES` env var.

**Golden test:** Run the full troubleshooting scenario against fixtures, diff output.

---

## Acceptance Criteria

1. `go vet && go test` all green
2. `MGTT_FIXTURES=fixtures/storefront-incident.yaml mgtt plan --model examples/storefront/system.model.yaml < /dev/null` produces correct troubleshooting output
3. `mgtt incident start` + `mgtt plan` + `mgtt fact add` + `mgtt incident end` workflow works
4. Fixture backend returns canned probe results
5. Parse modes handle all 8 formats
6. Golden test for plan loop passes
