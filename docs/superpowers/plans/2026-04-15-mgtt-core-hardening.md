# mgtt Core Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Lift cross-cutting provider-runner concerns (typed errors, timeouts, debug, caching, probe contract, SDK) into mgtt core so every provider gets them for free, and add core-side validation tooling that keeps providers honest.

**Architecture:** Extend the `internal/providersupport/probe` layer with a richer Result type (`status: ok|not_found`), a Runner result classifier, a shared Go SDK package exported at `sdk/provider`, and a new `mgtt provider validate` command. The SDK is consumed by external provider repos via `go get github.com/mgt-tool/mgtt/sdk/provider`. Fixture executor and engine UnresolvedError get wired to the new status field.

**Tech Stack:** Go 1.25, existing yaml.v3 only (no new deps), reuse existing expr/engine `UnresolvedError` path.

---

# Why this plan exists (written first — read before the tasks)

The sibling plan `mgtt-provider-kubernetes/docs/superpowers/plans/2026-04-15-provider-hardening.md` allocates ~50 tasks across 12 phases to harden one provider. Several of those tasks solve problems that belong in core, not in the provider:

## Cross-cutting audit

| Concern | Where solved today | Problem | Who fixes best |
|---|---|---|---|
| "Resource does not exist" handling | Each provider invents its own (or errors out) | Design-time simulations break; every new provider reinvents the wheel | **Core** — extend probe protocol with `status` field, translate to engine-level `UnresolvedError` |
| Typed error taxonomy (NotFound, Forbidden, Transient, Protocol, Env) | `internal/providersupport/probe/runner.go:30` wraps everything as opaque `fmt.Errorf` | Engine can't distinguish retriable from fatal; users see `exec: exit status 1` | **Core** — classifier on ExternalRunner + sentinel errors exposed through SDK |
| Per-probe timeout | `Command.Timeout` is populated but `ExternalRunner.Run` at `runner.go:30` never uses it | Runaway provider binary hangs mgtt forever | **Core** — 1-line fix in ExternalRunner |
| In-process kubectl dedup / caching | None | Provider binary is re-invoked per fact; each probe re-shells kubectl even for identical resources | **Provider** (mgtt invokes provider once per fact; dedup happens inside provider's process lifetime — SDK helps but is per-provider) |
| Debug/trace of probe round-trip | None | Users get no visibility into argv, duration, or result across the mgtt ↔ provider boundary | **Core** — `MGTT_DEBUG=1` traces every probe invocation from the mgtt side |
| Runner binary presence check | No central check — first probe fails opaquely | Confusing first-run experience after install | **Core** — verify `meta.command` (the provider's OWN runner binary) exists on load. Core knows nothing about kubectl/aws/docker; those are the provider's backend dependencies, detected by the provider (via the SDK's `shell.Client` returning `ErrEnv` on `exec.ErrNotFound`). |
| Provider protocol compliance | No test: docs in `providers/README.md` are advisory | Every provider re-reads the docs and gets the contract slightly wrong | **Core** — `mgtt provider validate <name>` does static + live checks |
| Runner argv construction | Fixed in `runner.go:22-28`: `probe <component> <fact> [--namespace NS] [--type TYPE]` — no way to pass GVK for CRDs | Blocks `custom_resource` probes cleanly | **Core** — extend Command with a `Extra map[string]string` that maps to repeated `--key value` flags |
| SDK / boilerplate reduction | None. Provider repos re-implement argv parsing, JSON encoding, status translation | ~150 lines of duplicated plumbing per provider | **Core** — ship `sdk/provider` Go module with `Main()`, `Registry`, `KubeClient`-like helpers |
| Fixture executor `not_found` support | `fixture/executor.go:50-65` has no way to express "missing" | Can't test design-time simulation paths involving missing resources | **Core** — extend fixture YAML to allow `status: not_found` entries |
| Parity between declared facts and runner-supported facts | None. Silent drift. | Docs-reality divergence | **Core** — `mgtt provider validate` runs each declared fact through runner |
| Version requirement (`requires: mgtt: ">=1.0"`) | Declared in `provider.yaml` but **not enforced** anywhere | A 2.0-only provider loads against a 1.0 mgtt and fails confusingly | **Core** — semver gate in provider loader |

## Concrete knock-on effect on the provider plan

If **this core plan** merges first, the sibling **provider-hardening plan** shrinks roughly as follows:

- **Phase 1 (architecture)** — Tasks 1.1–1.3 (`internal/kubeclient`) become `import "github.com/mgt-tool/mgtt/sdk/provider"` + thin registrations. ~70% shorter.
- **Phase 2 (robustness)** — Deleted entirely. SDK's `shell.Client` provides timeouts, size caps, and a generic `exec.ErrNotFound → ErrEnv` mapping that works for any backend CLI (kubectl, aws, docker). Core stays backend-agnostic; the provider's own `Classify` adds kubectl-specific stderr patterns (NotFound/Forbidden/etc.).
- **Phase 3 (observability)** — `doctor` subcommand absorbed by `mgtt provider validate`; provider keeps a minimal `version` subcommand only.
- **Phase 0 probe contract doc** — moved up here; canonical in core.
- **Phase 9 parity test** — deleted; `mgtt provider validate` in CI replaces it.
- **Provider-side Phases 4–8 (type coverage)** — unchanged; must still be written per-type.
- **Provider-side Phases 10–12 (install hook, release, sweep)** — unchanged.

Net: the provider plan drops from ~50 tasks to ~30, and every *future* provider (aws, docker, …) reuses the work.

## Decision point

Two paths — read both before starting:

- **(A) Core-first, then provider.** Implement this plan; revise provider plan to consume SDK; then execute the (slimmer) provider plan. Longer critical path; less total work; every future provider wins.
- **(B) Provider-first, fold core later.** Execute provider plan as written; extract duplicated plumbing into core SDK in a later cycle. Faster first kubernetes release; creates migration work later.

**Recommendation: (A).** The duplicated surface will grow as aws/docker providers are hardened next — paying it off now is cheaper.

If you pick (A), **the provider plan should be re-plotted after Phase 4 of this core plan lands** (the SDK shape is the hinge). I'll call out the exact re-plotting point at Phase 4's exit.

---

## File Structure

```
mgtt/
  internal/
    providersupport/
      load.go                               # modify: enforce requires.mgtt semver
      probe/
        types.go                            # modify: Result adds Status; Command adds Extra
        runner.go                           # modify: classify errors, honor Timeout, parse Status
        errors.go                           # new: ErrNotFound/Forbidden/Transient/Protocol/Env
        errors_test.go                      # new
        trace.go                            # new: MGTT_DEBUG tracer
        trace_test.go                       # new
        runner_test.go                      # new: classifier + timeout + argv tests
        fixture/
          executor.go                       # modify: parse status: not_found entries
          executor_test.go                  # modify
      validate/
        validate.go                         # new: static + live validation
        validate_test.go                    # new
    engine/
      plan.go                               # modify: surface status: not_found as UnresolvedError
      engine_test.go                        # modify: add not_found propagation test
    cli/
      provider_validate.go                  # new: `mgtt provider validate <name>` command
      provider_validate_test.go             # new
      root.go                               # modify: register validate subcommand

  sdk/
    provider/
      doc.go                                # package overview
      main.go                               # sdk.Main(registrations) — flag parsing, dispatch
      main_test.go
      result.go                             # Result, Status constants
      registry.go                           # type/fact registry
      registry_test.go
      errors.go                             # sentinel errors (mirrored from internal for external import)
      runtime_test.go                       # integration-style test using sdk.Main
      shell/
        client.go                           # optional helper: command runner with typed errors, size cap, timeout
        client_test.go
      README.md                             # how to write a provider using this SDK

  docs/
    PROBE_PROTOCOL.md                       # new: authoritative probe contract (supersedes per-provider docs)
    providers/
      README.md                             # modify: point to sdk/provider and PROBE_PROTOCOL.md
```

---

## Scope & Non-Goals

**In scope:**
- Extend probe protocol with `status` field + wire through engine
- Typed errors + classifier in ExternalRunner
- Honor `Command.Timeout` in ExternalRunner
- `MGTT_DEBUG=1` tracing on the probe boundary
- Runner binary presence check on provider load
- `mgtt provider validate <name>` command (static + live)
- Version requirement enforcement in loader
- `sdk/provider` Go package (public, consumable by external provider repos)
- Authoritative `docs/PROBE_PROTOCOL.md`
- Extend fixture executor with `status: not_found`
- `Command.Extra` map for arbitrary `--flag value` pairs (unblocks CRDs)

**Non-goals (YAGNI):**
- Binary protocol / gRPC between mgtt and provider runner — JSON-over-argv is sufficient
- Hot-reload of providers
- Provider signature verification (deferred; not part of v0 threat model)
- Streaming probe results (single-shot JSON is enough)
- Retry policy in core (retry is the caller's concern; core just classifies Transient)
- Parallel probe execution (engine decides; provider stays sequential)
- **OS-level sandboxing of provider runners** — v0 enforcement relies on operator-controlled credentials (kubernetes RBAC, AWS IAM, etc.) matching `auth.access.writes`. Kernel sandboxing (seccomp, namespaces, sandbox-exec) is platform-specific, fights tool assumptions (kubectl writes to `~/.kube/cache`), and is deferred to a future iteration.

---

## Adversarial Review (2026-04-16) — MUST READ BEFORE EXECUTION

An independent review flagged the following. Critical items are fixed inline below; important items have new guards; suggestions are captured for triage.

### Critical — fixed inline; do not execute the plan until you have verified these apply

- **[C1] Fixture Status defaulting must precede engine translation.** Re-ordered: Task 2.2 (fixture defaults) now runs *before* Task 2.1 (engine wiring). The fixture executor explicitly sets `Status: StatusOk` on successful parse, not only in the new not-found branch.
- **[C2] Task 1.2 must patch `internal/cli/plan.go:177,186` in the same commit.** Existing code declares `runners := map[string]*probe.ExternalRunner{}` and assigns to the Mux. A `map[string]*ExternalRunner` is NOT assignable to `map[string]Executor` (Go invariance). Patching both lines is now a mandatory step in Task 1.2.
- **[C3] The sibling provider plan is stale and duplicates SDK work.** Do NOT start `mgtt-provider-kubernetes/docs/superpowers/plans/2026-04-15-provider-hardening.md` until that file is physically edited per the "Revised Provider-Hardening Plan Diff" section of this plan. Leaving the revision in prose is a coordination failure — agentic workers will follow the literal text.
- **[C4] SDK must be verifiably importable.** Scratch-directory `go get github.com/mgt-tool/mgtt/sdk/provider@vX.Y.Z && go build` against a 10-line main is promoted from a sweep checklist item to a release gate before Task 7.2. Additionally, `go test -mod=mod ./sdk/...` must pass with no imports of `internal/`.

### Important — new guards added below

- **[I1] Test helpers in Task 1.2 use `strconv.Itoa`, not the broken inline `itoa`.** The plan's test code has been corrected; the parenthetical caveat is removed.
- **[I2] `Extra` key collisions with `Vars`.** Runner rejects colliding keys with `ErrUsage`. PROBE_PROTOCOL.md documents the rule.
- **[I3] Argv ordering audit.** Task 1.3 adds a pre-commit grep for in-tree tests that assert exact argv; any hit is migrated before the runner change lands.
- **[I4] Trace format has no backend vocabulary.** `TraceStart` prints `vars=N extra=N` counts, not `namespace=`. The full map goes to a separate line only when a verbose flag is set — never printed by default.
- **[I5] `validate --live` is explicitly excluded from core CI.** Documented in the validate task and in PROBE_PROTOCOL.md. Each provider's CI owns the `--live` invocation against its own test fixture.
- **[I6] Semver parser pulls `golang.org/x/mod/semver`.** That is a stdlib-adjacent module, not a new runtime dep in spirit. Restricts supported ranges to the standard semver grammar; rejects exotic forms at load time with a clear error.

### Suggestions — captured for triage, not all adopted

- **[S1] MGTT_DRY_RUN cut from v0.** No concrete user. Revisit after first operator request.
- **[S2] CREDENTIALS.md convention kept as documentation; no validate warning.** Documentation in `sdk/provider/README.md` is sufficient; a warning on every provider that doesn't ship the file is noise.
- **[S3] Provider plan's doctor subcommand is DELETED, not rewritten,** as part of the C3 revision.
- **[S4] Provider plan's "37 types" goal is triaged.** Tier 1 must-implement (~10): deployment, statefulset, daemonset, pod, service, endpoints, ingress, pvc, node, hpa. Tier 2 nice-to-have (~10): namespace, configmap, secret, job, cronjob, pdb, networkpolicy, serviceaccount, role, rolebinding. Tier 3 deferred: webhooks, CSI, leases, priorityclass, extensibility. The parity test marks Tier 2+ entries permanently exempt with a pointer to the triage doc.
- **[S5] UnresolvedError UX check.** New task in Phase 2: write a scenario test asserting operator-visible output when a probed resource is missing. "Resource X not found in namespace Y" must surface, not be swallowed as "unknown state."

### Missing — new items added below

- **[M1] Uninstall must not be gated by the semver check.** Task 5.1 explicitly preserves `mgtt provider uninstall <name>` as always-possible regardless of version mismatch. An incompatible provider must still be removable.
- **[M2] Cache observability.** Debug log at hit/miss boundary in the SDK `Caching` layer — lives in the provider plan, not here, but is added to the "Revised Provider-Hardening Plan Diff" section.
- **[M3] Error message quality gate.** Phase 8 cross-cutting sweep adds a manual checklist item: exercise each sentinel path, assert messages are ≤80 chars with resource name and next action.

### Sequencing note

Corrected execution order:
1. Phase 0 → Phase 1 (with C2 patch bundled) → **Phase 2: Task 2.2 before 2.1** (C1) → Phase 3 (with I4 trace fix) → Phase 4 (SDK) parallelized with Phase 5 (loader hardening) → Phase 6 (validate, static only in core CI) → Phase 7.2 release **gated on C4 scratch-dir test** → edit provider plan per C3 → provider plan execution.

---

## Layering Invariant (audit every task against this)

**Core talks to providers through interfaces and data, never concrete types or backend-specific names.**

Concretely:
- The `probe.Executor` interface is the only contract between mgtt's engine/CLI and any provider. Every dispatch path goes through it.
- `Mux.Runners` map value type is `Executor`, not `*ExternalRunner`. Tests and alternative backends plug in without modifying core.
- No core code knows the word "kubectl", "aws", "docker", "namespace", "cluster", "region", or any other backend vocabulary. Special-casing any key (e.g. hardcoding `--namespace` in runner argv) is a layering violation.
- `Command.Vars` and `Command.Extra` are opaque maps. Core passes them through; it never inspects or filters keys.
- Error sentinels (`ErrNotFound`, `ErrForbidden`, …) are part of the protocol, mapped by **exit code** (provider-declared via the protocol), NOT by matching stderr strings. String matching for error classification lives at the SDK layer where each provider supplies its own `Classify` function — never in core.
- `Result.Status` values (`"ok"`, `"not_found"`) are protocol constants, not backend concepts.

Every task below is checked against this invariant. Where it would have been violated, a "Layering check" bullet explains the fix.

## Decision Log (locked before planning)

| Decision | Chosen | Rejected | Why |
|---|---|---|---|
| Status field location | `Result.Status string` with constants `"ok"`, `"not_found"` | New top-level field outside Result | Backward compatible: older providers that don't emit status default to "ok" via json zero value + one-line upgrade in runner |
| Error plumbing | Sentinel errors wrapped via `fmt.Errorf("%w: %s", ErrX, msg)` exposed from a new `internal/providersupport/probe/errors.go` and re-exported from `sdk/provider/errors.go` | Numeric error codes in JSON | Go idiomatic; `errors.Is` works across module boundaries when SDK re-exports |
| SDK module path | `github.com/mgt-tool/mgtt/sdk/provider` (under mgtt repo) | Separate repo `mgtt-provider-sdk` | Version-locked to mgtt release; easier to co-evolve protocol and SDK; one tag to coordinate |
| SDK API shape | `sdk.Main(types map[string]sdk.TypeHandler)` — providers register in main() | Auto-discovery via init() | Explicit > magic; init-based registration doesn't surface missing imports at compile time |
| `Command.Extra` encoding | Each key becomes `--key value` flag pair, ordered by key | JSON blob env var | Keeps the runner protocol shell-visible and debuggable |
| Validate command live mode | Opt-in via `--live` flag; static check runs always | Always live | Static parse + exempt-map check catches 90% of drift; live requires cluster |
| UnresolvedError translation | `status: not_found` in probe result → `UnresolvedError{Component, Fact, Reason: "resource not found"}` surfaced by engine | Drop the probe silently | Engine already handles UnresolvedError via errors.As; no new plumbing |

---

# PHASE 0: Foundation

## Task 0.1: Write PROBE_PROTOCOL.md

**Files:**
- Create: `docs/PROBE_PROTOCOL.md`

- [ ] **Step 1: Write the contract**

```markdown
# Probe Protocol

This document is the authoritative contract between mgtt and any provider runner binary. Providers MUST conform. mgtt MUST NOT assume behavior not specified here.

## Invocation

mgtt invokes the runner as:

    <runner> probe <component-name> <fact-name> [--namespace NS] [--type TYPE] [--key value ...]

- `<runner>` is `meta.command` from provider.yaml.
- Additional `--key value` pairs come from `Command.Extra` — providers that need out-of-band data (CRD GVK, region, cluster, etc.) declare which flags they expect in their README.

## Success output (stdout, exit 0)

A single JSON object, one line, followed by newline:

    {"value": <typed value or null>, "raw": "<human-readable>", "status": "ok"|"not_found"}

- `value` matches the declared fact type.
- `raw` is a short operator-friendly rendering.
- `status`:
  - `"ok"` — authoritative value.
  - `"not_found"` — the resource does not exist. `value` MUST be null, `raw` MAY be empty. Core translates this to an engine `UnresolvedError`. Providers MUST use this instead of exit 1 for missing resources.

If `status` is omitted, core defaults it to `"ok"` (backward compatibility with 1.x providers).

## Error output (stderr, non-zero exit)

A single human-readable line on stderr, then exit code per table:

| Exit | Class | Meaning |
|---|---|---|
| 0 | success | Probe succeeded (including not_found) |
| 1 | usage | Bad args, unknown type/fact |
| 2 | env | Required dependency missing (kubectl, aws CLI, …) |
| 3 | forbidden | RBAC / auth rejection |
| 4 | transient | Network, timeout, 5xx — caller may retry |
| 5 | protocol | Backend returned malformed data |

Core maps exit codes to sentinel errors:
- `1 → ErrUnknownFact` / `ErrUnknownType`
- `2 → ErrEnv`
- `3 → ErrForbidden`
- `4 → ErrTransient`
- `5 → ErrProtocol`

## Timeouts and limits

- Each probe is bounded by `Command.Timeout` (default 30s). Exceeding the timeout sends SIGTERM, then SIGKILL after 2s.
- Runner stdout >10 MiB is truncated and the call is treated as `ErrProtocol`.

## Debug output

When mgtt sets `MGTT_DEBUG=1` in the runner's environment, providers MAY emit trace lines to stderr. Debug MUST NOT be written to stdout — it would corrupt the JSON.

## Versioning

`provider.yaml` declares `meta.requires.mgtt` as a semver range. Core refuses to load incompatible providers. Providers that want to use `Command.Extra` MUST declare `requires: mgtt: ">=1.1"`.
```

- [ ] **Step 2: Commit**

```bash
git add docs/PROBE_PROTOCOL.md
git commit -m "docs: authoritative probe protocol — supersedes per-provider docs"
```

---

# PHASE 1: Error Taxonomy + Timeout Fix

## Task 1.1: Add sentinel errors

**Files:**
- Create: `internal/providersupport/probe/errors.go`
- Create: `internal/providersupport/probe/errors_test.go`

- [ ] **Step 1: Write failing test**

```go
package probe

import (
	"errors"
	"testing"
)

func TestClassifyExit_MapsExitCodes(t *testing.T) {
	cases := []struct {
		code int
		want error
	}{
		{1, ErrUsage},
		{2, ErrEnv},
		{3, ErrForbidden},
		{4, ErrTransient},
		{5, ErrProtocol},
		{99, ErrUnknown},
	}
	for _, c := range cases {
		got := ClassifyExit(c.code, "stderr msg")
		if !errors.Is(got, c.want) {
			t.Errorf("exit %d: want %v, got %v", c.code, c.want, got)
		}
	}
}

func TestClassifyExit_IncludesStderrMessage(t *testing.T) {
	err := ClassifyExit(3, "forbidden: user x cannot get deployments")
	if err == nil || !containsString(err.Error(), "forbidden: user x") {
		t.Fatalf("err should include stderr: %v", err)
	}
}

func containsString(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run — expect failure**

Run: `go test ./internal/providersupport/probe/ -run ClassifyExit`
Expected: FAIL — undefined.

- [ ] **Step 3: Implement**

`internal/providersupport/probe/errors.go`:

```go
package probe

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrUsage     = errors.New("provider: usage error")
	ErrEnv       = errors.New("provider: environment error")
	ErrForbidden = errors.New("provider: forbidden")
	ErrTransient = errors.New("provider: transient error")
	ErrProtocol  = errors.New("provider: protocol error")
	ErrUnknown   = errors.New("provider: unknown error")
)

// ClassifyExit maps a runner exit code to a sentinel error wrapped with the
// provider's stderr message. See docs/PROBE_PROTOCOL.md for the spec.
func ClassifyExit(code int, stderr string) error {
	msg := firstLine(stderr)
	switch code {
	case 1:
		return fmt.Errorf("%w: %s", ErrUsage, msg)
	case 2:
		return fmt.Errorf("%w: %s", ErrEnv, msg)
	case 3:
		return fmt.Errorf("%w: %s", ErrForbidden, msg)
	case 4:
		return fmt.Errorf("%w: %s", ErrTransient, msg)
	case 5:
		return fmt.Errorf("%w: %s", ErrProtocol, msg)
	}
	return fmt.Errorf("%w: exit %d: %s", ErrUnknown, code, msg)
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}
```

- [ ] **Step 4: Run — expect pass**

Run: `go test ./internal/providersupport/probe/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/providersupport/probe/errors.go internal/providersupport/probe/errors_test.go
git commit -m "feat(probe): sentinel errors + exit-code classifier"
```

## Task 1.2: Honor Command.Timeout in ExternalRunner + capture stderr + classify

**Files:**
- Modify: `internal/providersupport/probe/runner.go`
- Modify: `internal/providersupport/probe/types.go`
- **Modify: `internal/cli/plan.go`** — lines 177 and 186 change to use the Executor interface in the map (see Step 4.5 below)
- Create: `internal/providersupport/probe/runner_test.go`

- [ ] **Step 1: Write failing tests**

```go
package probe

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// writeFakeRunner writes a tiny bash script that emits given stdout+stderr+exit.
// On non-linux/darwin platforms, the test is skipped.
func writeFakeRunner(t *testing.T, stdout, stderr string, exit int) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("bash-based fake runner not supported on windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "fake-runner")
	script := "#!/bin/sh\n" +
		"printf '%s' " + shellQuote(stdout) + "\n" +
		"printf '%s' " + shellQuote(stderr) + " 1>&2\n" +
		"exit " + strconv.Itoa(exit) + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func shellQuote(s string) string { return "'" + s + "'" }

// Use strconv.Itoa in the real test file; add "strconv" to imports.

func TestExternalRunner_SuccessWithStatus(t *testing.T) {
	bin := writeFakeRunner(t, `{"value":3,"raw":"3","status":"ok"}`, "", 0)
	r := NewExternalRunner(bin)
	res, err := r.Run(context.Background(), Command{Provider: "p", Component: "c", Fact: "f"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "ok" {
		t.Fatalf("want ok, got %q", res.Status)
	}
	if res.Parsed != float64(3) && res.Parsed != 3 {
		t.Fatalf("parsed want 3, got %v (%T)", res.Parsed, res.Parsed)
	}
}

func TestExternalRunner_NotFoundStatus(t *testing.T) {
	bin := writeFakeRunner(t, `{"value":null,"raw":"","status":"not_found"}`, "", 0)
	r := NewExternalRunner(bin)
	res, err := r.Run(context.Background(), Command{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "not_found" {
		t.Fatalf("want not_found, got %q", res.Status)
	}
}

func TestExternalRunner_StatusDefaultsToOkWhenOmitted(t *testing.T) {
	bin := writeFakeRunner(t, `{"value":1,"raw":"1"}`, "", 0)
	r := NewExternalRunner(bin)
	res, err := r.Run(context.Background(), Command{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != StatusOk {
		t.Fatalf("backward compat: omitted status should default to ok, got %q", res.Status)
	}
}

func TestExternalRunner_ClassifyForbidden(t *testing.T) {
	bin := writeFakeRunner(t, "", "rbac denied", 3)
	r := NewExternalRunner(bin)
	_, err := r.Run(context.Background(), Command{})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestExternalRunner_Timeout(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "slow-runner")
	os.WriteFile(path, []byte("#!/bin/sh\nsleep 10\n"), 0o755)
	r := NewExternalRunner(path)
	start := time.Now()
	_, err := r.Run(context.Background(), Command{Timeout: 100 * time.Millisecond})
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected error")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("runner ignored timeout; elapsed %v", elapsed)
	}
}

func TestExternalRunner_ProtocolErrorOnBadJSON(t *testing.T) {
	bin := writeFakeRunner(t, "not json", "", 0)
	r := NewExternalRunner(bin)
	_, err := r.Run(context.Background(), Command{})
	if !errors.Is(err, ErrProtocol) {
		t.Fatalf("want ErrProtocol, got %v", err)
	}
}
```

(The `strconv` import is required in the test file.)

- [ ] **Step 2: Run — expect failure**

- [ ] **Step 3: Rewrite `runner.go`**

```go
package probe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	StatusOk       = "ok"
	StatusNotFound = "not_found"
)

type ExternalRunner struct {
	Binary string
}

func NewExternalRunner(binary string) *ExternalRunner {
	return &ExternalRunner{Binary: binary}
}

func (r *ExternalRunner) Run(ctx context.Context, cmd Command) (Result, error) {
	args := []string{"probe", cmd.Component, cmd.Fact}
	if cmd.Type != "" {
		args = append(args, "--type", cmd.Type)
	}
	// Layering invariant: core does not privilege any Vars key. Every entry in
	// Vars and Extra becomes --<key> <value> in sorted order. Providers decide
	// which keys they know about; core stays backend-agnostic.
	//
	// Key collision between Vars and Extra is a usage error — callers must
	// resolve it before calling Run, because silent precedence would hide bugs.
	for k := range cmd.Extra {
		if _, conflict := cmd.Vars[k]; conflict {
			return Result{}, fmt.Errorf("%w: key %q present in both Vars and Extra", ErrUsage, k)
		}
	}
	merged := map[string]string{}
	for k, v := range cmd.Vars {
		if v != "" {
			merged[k] = v
		}
	}
	for k, v := range cmd.Extra {
		if v != "" {
			merged[k] = v
		}
	}
	for _, k := range sortedKeys(merged) {
		args = append(args, "--"+k, merged[k])
	}

	timeout := cmd.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	c := exec.CommandContext(ctx, r.Binary, args...)
	var stderr strings.Builder
	c.Stderr = &stderr
	stdout, runErr := c.Output()

	if runErr != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return Result{}, fmt.Errorf("%w: runner %s exceeded %s", ErrTransient, r.Binary, timeout)
		}
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			return Result{}, ClassifyExit(exitErr.ExitCode(), stderr.String())
		}
		return Result{}, fmt.Errorf("%w: runner %s: %v", ErrEnv, r.Binary, runErr)
	}

	var rr struct {
		Value  any    `json:"value"`
		Raw    string `json:"raw"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(stdout, &rr); err != nil {
		return Result{}, fmt.Errorf("%w: parse runner output: %v", ErrProtocol, err)
	}
	if rr.Status == "" {
		rr.Status = StatusOk
	}
	return Result{Raw: rr.Raw, Parsed: rr.Value, Status: rr.Status}, nil
}

// Mux dispatches commands to a per-provider Executor, falling back to Default.
// Runners is keyed by provider name and typed as the Executor interface so
// callers (including tests) can inject any implementation — not just
// ExternalRunner. This preserves the core-via-interfaces invariant.
type Mux struct {
	Default Executor
	Runners map[string]Executor
}

func (m *Mux) Run(ctx context.Context, cmd Command) (Result, error) {
	if r, ok := m.Runners[cmd.Provider]; ok {
		return r.Run(ctx, cmd)
	}
	return m.Default.Run(ctx, cmd)
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// stdlib sort to avoid new deps
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j-1] > keys[j]; j-- {
			keys[j-1], keys[j] = keys[j], keys[j-1]
		}
	}
	return keys
}
```

- [ ] **Step 4: Update `types.go`**

```go
type Command struct {
	Raw       string
	Parse     string
	Provider  string
	Component string
	Fact      string
	Type      string
	Vars      map[string]string
	Extra     map[string]string // appended as --key value pairs to runner
	Timeout   time.Duration
}

type Result struct {
	Raw    string
	Parsed any
	Status string // "ok" (default) or "not_found"
}
```

- [ ] **Step 4.5: Patch `internal/cli/plan.go` (mandatory — won't compile otherwise)**

The existing code at `plan.go:177`:

```go
runners := map[string]*probe.ExternalRunner{}
```

Change to:

```go
runners := map[string]probe.Executor{}
```

And at `plan.go:186`:

```go
return &probe.Mux{Default: probeexec.Default(), Runners: runners}, nil
```

The line text is unchanged, but now assigns the interface-typed map into the interface-typed `Mux.Runners` field. The variable at line 177 is the only thing that changed.

Grep to confirm no other call site constructs the old map type:

```bash
grep -rn "map\[string\]\*probe\.ExternalRunner" .
```

Expected: zero hits after this step.

- [ ] **Step 5: Run all tests — expect existing tests to still pass**

Run: `go test ./...`
Expected: PASS. If any test breaks on `Result.Status` absence, set it to `"ok"` in test fixtures — that's the one-line migration per the protocol.

- [ ] **Step 6: Commit**

```bash
git add internal/providersupport/probe/
git commit -m "feat(probe): Status field, timeout enforcement, error classification, generic Vars/Extra passthrough"
```

**Layering check:** Before this task, `runner.go:23-24` hardcoded `--namespace` from `cmd.Vars["namespace"]`. After this task, all Vars entries are passed as `--<key> <value>` pairs, treating "namespace" as no more special than any other model variable. Kubernetes-specific knowledge stays in the kubernetes provider's argv parser.

## Task 1.3: Update ExternalRunner callers to not rely on `--namespace` being free

**Files:**
- Audit: `internal/engine/*.go` for any assumption about namespace being magic.
- Audit: existing integration tests.

- [ ] **Step 1: Grep for namespace assumptions AND argv-order assertions (I3)**

Run, in order:

```bash
grep -rn '"namespace"' internal/ --include='*.go' | grep -v testdata
grep -rn 'args\[\|Args\[' internal/providersupport/probe/ --include='*_test.go'
grep -rn '--namespace' internal/ --include='*.go' | grep -v testdata
```

Expected for the first: just the loader, surfacing `variables.namespace` from provider.yaml into the model as a generic variable. The engine treats it as any other variable.

Expected for the second and third: any test that asserts on argv order or the literal `--namespace` flag is now coupled to the removed special case. Migrate each such test before the runner rewrite lands — either assert on argv-as-a-set rather than argv-as-a-list, or use a wildcard matcher.

If any test remains order-sensitive after migration, document why in a comment next to the assertion.

- [ ] **Step 2: Verify the kubernetes provider's runner still parses `--namespace` correctly**

Its `main.go` already reads `--namespace`; no change needed there. The only behavior shift is that now `--type` appears *before* other flags in the argv (alphabetical). The kubernetes provider's flag parser is loop-based and order-insensitive, so this is safe.

- [ ] **Step 3: Run full integration tests**

```bash
MGTT_IMAGE=... go test -tags=integration ./...
```

Expected: green.

- [ ] **Step 4: Commit (if any adjustments were needed)**

**Layering check:** The engine never knew "namespace" was special; only `runner.go` did. Removing the special case moves the concept entirely into the kubernetes provider.

---

# PHASE 2: Wire `not_found` Through Engine

**Order correction (per C1):** Task 2.2 (fixture Status default) runs FIRST so every existing test fixture deterministically returns `Status: StatusOk` before Task 2.1 teaches the engine to branch on it. Running them in the opposite order produces tests that pass by accident.

## Task 2.1 (runs SECOND): Surface not_found as UnresolvedError

**Files:**
- Modify: `internal/engine/plan.go`
- Modify: `internal/engine/engine_test.go`

- [ ] **Step 1: Identify the call site**

Run: `grep -n "Run(" internal/engine/plan.go`. Find the place where probe execution results are consumed. Add handling: if `result.Status == probe.StatusNotFound`, convert to `expr.UnresolvedError{Component, Fact, Reason: "resource not found"}`.

- [ ] **Step 2: Write failing test in `engine_test.go`**

```go
func TestEngine_NotFoundProducesUnresolvedError(t *testing.T) {
	// Build a minimal model with one component, probe returns status:not_found
	// Assert: engine yields UnresolvedError{Component:"x", Fact:"ready_replicas", Reason: "resource not found"}
	// (full setup per existing engine_test patterns)
}
```

- [ ] **Step 3: Run — expect failure**

- [ ] **Step 4: Implement the translation in plan.go**

At the probe-result consumption point:

```go
res, err := executor.Run(ctx, cmd)
if err != nil {
    return ...
}
if res.Status == probe.StatusNotFound {
    return expr.UnresolvedError{Component: cmd.Component, Fact: cmd.Fact, Reason: "resource not found"}
}
```

- [ ] **Step 5: Run — expect pass**

- [ ] **Step 6: Commit**

```bash
git add internal/engine/ internal/providersupport/probe/
git commit -m "feat(engine): translate probe status:not_found to UnresolvedError"
```

## Task 2.3: Operator-visible scenario test for missing resource (S5)

**Files:**
- Create: `internal/engine/missing_resource_scenario_test.go`

The UnresolvedError path must surface "resource X not found" to the operator, not swallow it as "unknown state." Write a scenario test that:
1. Models a deployment with ready_replicas fact.
2. Fixture returns `status: not_found`.
3. Runs the engine/CLI output path end-to-end.
4. Asserts the rendered output contains the component name and the phrase "not found" (or equivalent operator-facing text).

If the assertion fails, the fix is in the CLI render layer, not in core — but catch it here before it ships.

Commit:

```bash
git add internal/engine/missing_resource_scenario_test.go
git commit -m "test(engine): missing-resource scenario surfaces resource name to operator"
```

## Task 2.2 (runs FIRST): Fixture executor supports `status: not_found` AND defaults Status on every successful parse

**Critical (C1):** the fixture executor today returns `Result{}` with `Status = ""` on success. The engine change in Task 2.1 branches on `StatusNotFound`, but a comparison like `res.Status != StatusOk` would mis-classify every legacy fixture as anomalous. This task explicitly sets `Status: StatusOk` on the success path, not only in the new `not_found` branch.

**Files:**
- Modify: `internal/providersupport/probe/fixture/executor.go`
- Modify: `internal/providersupport/probe/fixture/executor_test.go`

- [ ] **Step 1: Extend fixture YAML shape**

New optional field per entry:

```yaml
kubernetes:
  api-deploy:
    ready_replicas:
      stdout: "3\n"
      exit: 0
      status: not_found  # optional; omitted ⇒ "ok"
```

- [ ] **Step 2: Write failing test**

```go
func TestFixture_NotFoundStatus(t *testing.T) {
	// Load fixture with status: not_found
	// Assert Result.Status == probe.StatusNotFound, Parsed is nil
}
```

- [ ] **Step 3: Extend `fixtureEntry` struct**

```go
type fixtureEntry struct {
	Stdout string `yaml:"stdout"`
	Exit   int    `yaml:"exit"`
	Status string `yaml:"status"`
}
```

In `Run`:

```go
if entry.Status == probe.StatusNotFound {
    return probe.Result{Raw: entry.Stdout, Parsed: nil, Status: probe.StatusNotFound}, nil
}
parsed, err := probe.ParseOutput(cmd.Parse, entry.Stdout, entry.Exit)
if err != nil {
    return probe.Result{Raw: entry.Stdout}, err
}
return probe.Result{Raw: entry.Stdout, Parsed: parsed, Status: probe.StatusOk}, nil
```

- [ ] **Step 4: Run — expect pass**

- [ ] **Step 5: Commit**

```bash
git add internal/providersupport/probe/fixture/
git commit -m "feat(fixture): support status: not_found for simulation tests"
```

---

# PHASE 3: MGTT_DEBUG Tracing

## Task 3.1: Probe boundary tracer

**Files:**
- Create: `internal/providersupport/probe/trace.go`
- Create: `internal/providersupport/probe/trace_test.go`
- Modify: `internal/providersupport/probe/runner.go`

- [ ] **Step 1: Write failing test**

```go
package probe

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestTracer_WritesInvocationAndDuration(t *testing.T) {
	var buf bytes.Buffer
	tr := &Tracer{Enabled: true, W: &buf}
	ctx := WithTracer(context.Background(), tr)

	// Simulate trace points
	TraceStart(ctx, "fakebin", Command{Component: "c", Fact: "f"})
	time.Sleep(5 * time.Millisecond)
	TraceEnd(ctx, "fakebin", Result{Status: "ok"}, nil)

	out := buf.String()
	if !containsString(out, "c.f") || !containsString(out, "ok") {
		t.Fatalf("trace output missing pieces: %q", out)
	}
}

func TestTracer_SilentWhenDisabled(t *testing.T) {
	var buf bytes.Buffer
	tr := &Tracer{Enabled: false, W: &buf}
	ctx := WithTracer(context.Background(), tr)
	TraceStart(ctx, "b", Command{})
	if buf.Len() != 0 {
		t.Fatalf("expected silence, got %q", buf.String())
	}
}
```

- [ ] **Step 2: Implement**

`trace.go`:

```go
package probe

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"
)

type Tracer struct {
	Enabled bool
	W       io.Writer
}

func NewTracer() *Tracer {
	return &Tracer{
		Enabled: os.Getenv("MGTT_DEBUG") == "1",
		W:       os.Stderr,
	}
}

type ctxKey struct{}

func WithTracer(ctx context.Context, t *Tracer) context.Context {
	return context.WithValue(ctx, ctxKey{}, t)
}

func tracerFrom(ctx context.Context) *Tracer {
	if t, ok := ctx.Value(ctxKey{}).(*Tracer); ok {
		return t
	}
	return nil
}

type startMarker struct{}

func TraceStart(ctx context.Context, binary string, cmd Command) {
	t := tracerFrom(ctx)
	if t == nil || !t.Enabled {
		return
	}
	// Layering invariant (I4): do not name backend-specific keys like
	// "namespace" here. Print counts only; a verbose mode can dump the full
	// map if operators need it.
	fmt.Fprintf(t.W, "[mgtt %s] probe start: %s %s.%s (type=%s vars=%d extra=%d)\n",
		time.Now().Format("15:04:05.000"), binary, cmd.Component, cmd.Fact, cmd.Type,
		len(cmd.Vars), len(cmd.Extra))
}

func TraceEnd(ctx context.Context, binary string, res Result, err error) {
	t := tracerFrom(ctx)
	if t == nil || !t.Enabled {
		return
	}
	if err != nil {
		fmt.Fprintf(t.W, "[mgtt %s] probe end: %s err=%v\n",
			time.Now().Format("15:04:05.000"), binary, err)
		return
	}
	fmt.Fprintf(t.W, "[mgtt %s] probe end: %s status=%s parsed=%v\n",
		time.Now().Format("15:04:05.000"), binary, res.Status, res.Parsed)
}
```

- [ ] **Step 3: Wire into ExternalRunner.Run**

At the top of `Run`:

```go
TraceStart(ctx, r.Binary, cmd)
defer func() { TraceEnd(ctx, r.Binary, result, returnedErr) }()
```

(Convert `Run` to named returns for the defer to see them, or wrap the body.)

- [ ] **Step 4: Initialize tracer in CLI entry point**

In `internal/cli/root.go` or wherever the executor is constructed, wrap context:

```go
ctx = probe.WithTracer(ctx, probe.NewTracer())
```

- [ ] **Step 5: Run tests — expect pass**

- [ ] **Step 6: Manually verify**

```bash
MGTT_DEBUG=1 mgtt status some-component 2>&1 | grep "probe start"
```

Expected: one line per probe invocation.

- [ ] **Step 7: Commit**

```bash
git add internal/providersupport/probe/trace.go internal/providersupport/probe/trace_test.go internal/providersupport/probe/runner.go internal/cli/root.go
git commit -m "feat(probe): MGTT_DEBUG=1 traces probe invocation and result"
```

---

# PHASE 4: SDK Package (`sdk/provider`)

This phase defines the provider-facing Go SDK. **After this phase lands, the k8s-provider plan should be re-plotted to consume the SDK.**

## Task 4.1: Package skeleton + Result/Registry mirror

**Files:**
- Create: `sdk/provider/doc.go`
- Create: `sdk/provider/result.go`
- Create: `sdk/provider/result_test.go`
- Create: `sdk/provider/errors.go`
- Create: `sdk/provider/registry.go`
- Create: `sdk/provider/registry_test.go`

- [ ] **Step 1: Write doc.go**

```go
// Package provider is the SDK for building mgtt provider runner binaries.
//
// A minimal provider:
//
//	package main
//
//	import "github.com/mgt-tool/mgtt/sdk/provider"
//
//	func main() {
//	    r := provider.NewRegistry()
//	    r.Register("deployment", map[string]provider.ProbeFn{
//	        "ready_replicas": func(ctx context.Context, req provider.Request) (provider.Result, error) {
//	            // ... shell out, parse, return
//	        },
//	    })
//	    provider.Main(r)
//	}
//
// See docs/PROBE_PROTOCOL.md in the mgtt repo for the wire contract.
package provider
```

- [ ] **Step 2: Write result.go**

```go
package provider

const (
	StatusOk       = "ok"
	StatusNotFound = "not_found"
)

type Result struct {
	Value  any    `json:"value"`
	Raw    string `json:"raw"`
	Status string `json:"status,omitempty"`
}

func IntResult(v int) Result     { return Result{Value: v, Raw: itoa(v), Status: StatusOk} }
func BoolResult(v bool) Result   { return Result{Value: v, Raw: boolstr(v), Status: StatusOk} }
func StringResult(v string) Result { return Result{Value: v, Raw: v, Status: StatusOk} }
func NotFound() Result           { return Result{Value: nil, Raw: "", Status: StatusNotFound} }

// helpers (no fmt import to keep SDK lean is a micro-concern; fine to use fmt)
func itoa(n int) string { return fmt.Sprintf("%d", n) }
func boolstr(b bool) string {
	if b { return "true" }
	return "false"
}
```

(Add `import "fmt"`.)

- [ ] **Step 3: Write errors.go**

```go
package provider

import "errors"

var (
	ErrUsage     = errors.New("provider: usage error")
	ErrEnv       = errors.New("provider: environment error")
	ErrForbidden = errors.New("provider: forbidden")
	ErrTransient = errors.New("provider: transient error")
	ErrProtocol  = errors.New("provider: protocol error")
	ErrUnknown   = errors.New("provider: unknown error")
	ErrNotFound  = errors.New("provider: not found") // providers return this; SDK converts to Result{Status: not_found}
)
```

- [ ] **Step 4: Write registry.go**

```go
package provider

import (
	"context"
	"errors"
	"fmt"
)

type Request struct {
	Type      string
	Name      string
	Namespace string
	Fact      string
	Extra     map[string]string
}

type ProbeFn func(ctx context.Context, req Request) (Result, error)

type Registry struct {
	types map[string]map[string]ProbeFn
}

func NewRegistry() *Registry { return &Registry{types: map[string]map[string]ProbeFn{}} }

func (r *Registry) Register(typ string, facts map[string]ProbeFn) { r.types[typ] = facts }

func (r *Registry) Probe(ctx context.Context, req Request) (Result, error) {
	facts, ok := r.types[req.Type]
	if !ok {
		return Result{}, fmt.Errorf("%w: unknown type %q", ErrUsage, req.Type)
	}
	fn, ok := facts[req.Fact]
	if !ok {
		return Result{}, fmt.Errorf("%w: type %q has no fact %q", ErrUsage, req.Type, req.Fact)
	}
	res, err := fn(ctx, req)
	if errors.Is(err, ErrNotFound) {
		return NotFound(), nil
	}
	if err != nil {
		return Result{}, err
	}
	if res.Status == "" {
		res.Status = StatusOk
	}
	return res, nil
}

// Types returns registered type names (for validate command introspection).
func (r *Registry) Types() []string {
	out := make([]string, 0, len(r.types))
	for k := range r.types { out = append(out, k) }
	return out
}

func (r *Registry) Facts(typ string) []string {
	facts := r.types[typ]
	out := make([]string, 0, len(facts))
	for k := range facts { out = append(out, k) }
	return out
}
```

- [ ] **Step 5: Write registry_test.go**

Mirror the earlier provider plan's registry tests (unknown type, unknown fact, dispatch, not-found translation).

- [ ] **Step 6: Run tests — expect pass**

```bash
go test ./sdk/provider/...
```

- [ ] **Step 7: Commit**

```bash
git add sdk/provider/
git commit -m "feat(sdk): provider SDK skeleton — Registry, Result, sentinel errors"
```

## Task 4.2: `sdk.Main` entrypoint

**Files:**
- Create: `sdk/provider/main.go`
- Create: `sdk/provider/main_test.go`

- [ ] **Step 1: Write failing test**

```go
package provider_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/mgt-tool/mgtt/sdk/provider"
)

func TestRun_SuccessfulProbe(t *testing.T) {
	r := provider.NewRegistry()
	r.Register("foo", map[string]provider.ProbeFn{
		"bar": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			return provider.IntResult(42), nil
		},
	})
	var stdout, stderr bytes.Buffer
	code := provider.Run(context.Background(), r,
		[]string{"probe", "name", "bar", "--type", "foo"},
		&stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}
	want := `{"value":42,"raw":"42","status":"ok"}` + "\n"
	if stdout.String() != want {
		t.Fatalf("stdout:\n got %q\nwant %q", stdout.String(), want)
	}
}

func TestRun_VersionSubcommand(t *testing.T) {
	var stdout bytes.Buffer
	code := provider.Run(context.Background(), provider.NewRegistry(),
		[]string{"version"}, &stdout, &stdout)
	if code != 0 { t.Fatal("expected 0") }
	// Version string comes from provider's main via a package var — check non-empty
}

func TestRun_NotFoundExitsZero(t *testing.T) {
	r := provider.NewRegistry()
	r.Register("foo", map[string]provider.ProbeFn{
		"bar": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			return provider.Result{}, provider.ErrNotFound
		},
	})
	var stdout, stderr bytes.Buffer
	code := provider.Run(context.Background(), r,
		[]string{"probe", "missing", "bar", "--type", "foo"},
		&stdout, &stderr)
	if code != 0 {
		t.Fatalf("not_found should exit 0, got %d", code)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"status":"not_found"`)) {
		t.Fatalf("missing status field: %s", stdout.String())
	}
}

func TestRun_ExitCodesMatchProtocol(t *testing.T) {
	r := provider.NewRegistry()
	r.Register("foo", map[string]provider.ProbeFn{
		"forbidden": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			return provider.Result{}, provider.ErrForbidden
		},
		"transient": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			return provider.Result{}, provider.ErrTransient
		},
		"env": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			return provider.Result{}, provider.ErrEnv
		},
	})
	cases := map[string]int{"forbidden": 3, "transient": 4, "env": 2}
	for fact, want := range cases {
		var stdout, stderr bytes.Buffer
		got := provider.Run(context.Background(), r,
			[]string{"probe", "x", fact, "--type", "foo"},
			&stdout, &stderr)
		if got != want {
			t.Errorf("fact %s: want exit %d, got %d stderr=%s", fact, want, got, stderr.String())
		}
	}
}
```

- [ ] **Step 2: Implement**

`sdk/provider/main.go`:

```go
package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

// Version is set by providers via ldflags.
var Version = "dev"

// Main is the standard entrypoint. Providers call it from main().
func Main(r *Registry) {
	code := Run(context.Background(), r, os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(code)
}

// Run is the testable core of Main.
func Run(ctx context.Context, r *Registry, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: <runner> probe <name> <fact> [--namespace NS] [--type TYPE] [--key value ...]")
		return 1
	}
	switch args[0] {
	case "version":
		fmt.Fprintln(stdout, Version)
		return 0
	case "probe":
		// continue below
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
		return 1
	}
	if len(args) < 3 {
		fmt.Fprintln(stderr, "probe requires <name> and <fact>")
		return 1
	}
	req := Request{
		Name:      args[1],
		Fact:      args[2],
		Namespace: "default",
		Extra:     map[string]string{},
	}
	for i := 3; i+1 < len(args); i += 2 {
		key := args[i]
		val := args[i+1]
		switch key {
		case "--namespace":
			req.Namespace = val
		case "--type":
			req.Type = val
		default:
			if len(key) > 2 && key[:2] == "--" {
				req.Extra[key[2:]] = val
			}
		}
	}
	res, err := r.Probe(ctx, req)
	if err != nil {
		return exitFor(err, stderr, err.Error())
	}
	if err := json.NewEncoder(stdout).Encode(res); err != nil {
		fmt.Fprintln(stderr, err)
		return 5
	}
	return 0
}

func exitFor(err error, stderr io.Writer, msg string) int {
	fmt.Fprintln(stderr, msg)
	switch {
	case errors.Is(err, ErrUsage):
		return 1
	case errors.Is(err, ErrEnv):
		return 2
	case errors.Is(err, ErrForbidden):
		return 3
	case errors.Is(err, ErrTransient):
		return 4
	case errors.Is(err, ErrProtocol):
		return 5
	}
	return 1
}
```

- [ ] **Step 3: Run tests — expect pass**

- [ ] **Step 4: Commit**

```bash
git add sdk/provider/main.go sdk/provider/main_test.go
git commit -m "feat(sdk): Main entrypoint with flag parsing, JSON output, exit code mapping"
```

## Task 4.3: SDK shell helper (optional common utilities)

**Files:**
- Create: `sdk/provider/shell/client.go`
- Create: `sdk/provider/shell/client_test.go`

- [ ] **Step 1: Minimal shell-executor helper**

Same shape as the kubeclient in the provider plan (typed errors, size cap, timeout, injectable exec function) but **generic** — not kubectl-specific. Providers shelling out to aws, docker, gh etc. all benefit.

```go
// Package shell is an optional SDK helper for providers that shell out to an
// external CLI (kubectl, aws, docker, ...). It handles timeouts, size caps,
// and maps stderr patterns to the sentinel errors the SDK exposes.
package shell

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/mgt-tool/mgtt/sdk/provider"
)

type Client struct {
	Binary   string
	Timeout  time.Duration
	MaxBytes int
	// Classify is REQUIRED. Each provider supplies its own stderr → sentinel
	// error mapping — the SDK never bakes in backend-specific phrasing (that
	// would be a layering leak: "NotFound" is kubectl, "AccessDenied" is AWS,
	// "permission denied" is POSIX, etc.). If nil, New() supplies
	// EnvOnlyClassify which handles only the backend-agnostic case
	// (binary not found on PATH).
	Classify func(stderr string, runErr error) error
	// Exec allows tests to inject a fake backend.
	Exec func(ctx context.Context, args ...string) (stdout, stderr []byte, err error)
}

const defaultMax = 10 * 1024 * 1024

func New(binary string) *Client {
	return &Client{
		Binary:   binary,
		Timeout:  30 * time.Second,
		MaxBytes: defaultMax,
		Classify: EnvOnlyClassify, // backend-agnostic default
		Exec: func(ctx context.Context, args ...string) ([]byte, []byte, error) {
			cmd := exec.CommandContext(ctx, binary, args...)
			var stderr strings.Builder
			cmd.Stderr = &stderr
			out, err := cmd.Output()
			return out, []byte(stderr.String()), err
		},
	}
}

func (c *Client) Run(ctx context.Context, args ...string) ([]byte, error) {
	if c.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.Timeout)
		defer cancel()
	}
	stdout, stderr, err := c.Exec(ctx, args...)
	if err != nil {
		return nil, c.Classify(string(stderr), err)
	}
	if c.MaxBytes > 0 && len(stdout) > c.MaxBytes {
		return nil, fmt.Errorf("%w: %s output %d bytes exceeds %d", provider.ErrProtocol, c.Binary, len(stdout), c.MaxBytes)
	}
	return stdout, nil
}

func (c *Client) RunJSON(ctx context.Context, args ...string) (map[string]any, error) {
	out, err := c.Run(ctx, args...)
	if err != nil { return nil, err }
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		return nil, fmt.Errorf("%w: parse json: %v", provider.ErrProtocol, err)
	}
	return m, nil
}

// EnvOnlyClassify is the backend-agnostic default. It handles ONE case: the
// backend CLI binary is not on PATH. Everything else falls through to
// ErrUnknown — providers should supply their own Classify for fine-grained
// typing of their backend's error phrasing.
//
// This is deliberate: any string-match here (NotFound, Forbidden, permission
// denied, AccessDenied, …) privileges a specific backend's vocabulary. The
// SDK is backend-agnostic; per-backend heuristics belong in each provider.
func EnvOnlyClassify(stderr string, runErr error) error {
	if errors.Is(runErr, exec.ErrNotFound) {
		return fmt.Errorf("%w: %v", provider.ErrEnv, runErr)
	}
	if runErr != nil {
		return fmt.Errorf("%w: %s", provider.ErrUnknown, firstLine(stderr))
	}
	return nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}
```

- [ ] **Step 2: Write tests**

Mirror the `Client_*` tests from the provider plan (NotFound, Forbidden, size cap, timeout, bad JSON).

- [ ] **Step 3: Run tests — expect pass**

- [ ] **Step 4: Commit**

```bash
git add sdk/provider/shell/
git commit -m "feat(sdk): shell helper — timeout, size cap, stderr-to-sentinel classification"
```

## Task 4.4: Write sdk/provider/README.md

**Files:**
- Create: `sdk/provider/README.md`

- [ ] **Step 1: Document how to write a provider using the SDK.**

Include: minimal example, common patterns (returning `ErrNotFound`, using `shell.Client`), how to build (`go build -ldflags "-X github.com/mgt-tool/mgtt/sdk/provider.Version=$(cat VERSION)"`), and a link to `docs/PROBE_PROTOCOL.md`.

- [ ] **Step 2: Commit**

```bash
git add sdk/provider/README.md
git commit -m "docs(sdk): authoring guide for provider runners"
```

**⏸ Re-plotting checkpoint.** At this point, the provider-hardening plan for mgtt-provider-kubernetes (and every future provider) should be revised to import `github.com/mgt-tool/mgtt/sdk/provider`. Specifically:

- Provider plan's `internal/kubeclient` package → replaced by `sdk/provider/shell.Client` with a custom `Classify` that knows kubectl NotFound/Forbidden phrasing.
- Provider plan's `internal/probes/registry.go` → replaced by `provider.NewRegistry()`.
- Provider plan's Phase 2 robustness tasks → deleted; SDK covers them.
- Provider plan's Phase 3 doctor/version → `version` covered by SDK; `doctor` covered by `mgtt provider validate`.
- Provider plan's Phase 9 parity test → replaced by `mgtt provider validate --live <name>` in CI (Phase 6 below).

---

# PHASE 5: Loader Hardening

## Task 5.1: Enforce `requires: mgtt: ">=X"`

**Files:**
- Modify: `internal/providersupport/load.go`
- Modify: `internal/providersupport/provider_test.go`

- [ ] **Step 1: Add mgtt version constant**

In a new file `internal/providersupport/version.go`:

```go
package providersupport

// MgttVersion is the running mgtt's protocol version advertised to providers.
// Major bump = breaking protocol change. Providers declare requires.mgtt as semver.
const MgttVersion = "1.1.0"
```

- [ ] **Step 2: Write failing test**

```go
func TestLoad_RejectsIncompatibleRequires(t *testing.T) {
	yamlSrc := `
meta:
  name: test
  version: 1.0.0
  requires:
    mgtt: ">=99.0"
  command: "$MGTT_PROVIDER_DIR/bin/x"
`
	_, err := LoadFromBytes([]byte(yamlSrc))
	if err == nil { t.Fatal("expected incompatible error") }
	if !strings.Contains(err.Error(), "requires mgtt") {
		t.Fatalf("error should mention version mismatch: %v", err)
	}
}

func TestLoad_AcceptsCompatibleRequires(t *testing.T) {
	yamlSrc := `
meta:
  name: test
  version: 1.0.0
  requires:
    mgtt: ">=1.0"
  command: "$MGTT_PROVIDER_DIR/bin/x"
`
	if _, err := LoadFromBytes([]byte(yamlSrc)); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}
```

- [ ] **Step 3: Use `golang.org/x/mod/semver`** (stdlib-adjacent, widely used, not a new runtime-dep smell).

```bash
go get golang.org/x/mod/semver
```

Add `meta.Requires map[string]string` to the raw type. Parse the declared constraint and compare:

```go
import "golang.org/x/mod/semver"

// normalize converts "1.1.0" → "v1.1.0" as required by the semver package.
func normalize(v string) string {
    if v == "" { return "" }
    if v[0] == 'v' { return v }
    return "v" + v
}

// satisfies accepts only the ">=X.Y.Z" form. Anything else returns an error
// at load time — restricting the grammar keeps the protocol predictable.
func satisfies(version, constraint string) error {
    constraint = strings.TrimSpace(constraint)
    if !strings.HasPrefix(constraint, ">=") {
        return fmt.Errorf("requires.mgtt constraint %q unsupported; only \">=X.Y.Z\" is accepted", constraint)
    }
    want := normalize(strings.TrimSpace(strings.TrimPrefix(constraint, ">=")))
    have := normalize(version)
    if !semver.IsValid(want) || !semver.IsValid(have) {
        return fmt.Errorf("invalid semver: have %q want %q", have, want)
    }
    if semver.Compare(have, want) < 0 {
        return fmt.Errorf("provider requires mgtt %s; running %s", constraint, version)
    }
    return nil
}
```

Document the supported constraint grammar in `docs/PROBE_PROTOCOL.md`: **exactly `>=X.Y.Z`**, nothing else. Ranges, carets, tildes rejected at load time.

- [ ] **Step 4: Run — expect pass**

- [ ] **Step 4.5: Uninstall is NOT gated by the semver check (M1)**

An incompatible provider must still be removable. The version gate applies to `mgtt status`, `mgtt simulate`, `mgtt provider inspect`, and anywhere the provider's runner would actually be invoked — but NOT to `mgtt provider uninstall`. Add a test:

```go
func TestLoad_IncompatibleProviderStillLoadsForUninstall(t *testing.T) {
    // LoadForUninstall bypasses the semver gate; LoadForUse applies it.
    // Verify both code paths exist and that uninstall tolerates a version mismatch.
}
```

Implement two loader entrypoints: `LoadForUse` (current behavior, enforces gate) and `LoadForUninstall` (skips gate). The uninstall CLI command uses the latter.

- [ ] **Step 5: Commit**

```bash
git add internal/providersupport/
git commit -m "feat(loader): enforce requires.mgtt semver; uninstall bypasses gate"
```

## Task 5.2: Verify runner binary exists on load

**Files:**
- Modify: `internal/providersupport/load.go`

- [ ] **Step 1: Write failing test**

```go
func TestLoad_WarnsWhenRunnerAbsent(t *testing.T) {
	// Load a provider whose meta.command points at a nonexistent path.
	// Assert: warning is returned in Provider.Warnings, not a fatal error
	// (load-time absence is fine — install hook builds the binary; warn only if
	// both the path and the install hook are missing).
}
```

- [ ] **Step 2: Add `Warnings []string` field to Provider**

At load time, if `meta.command` is non-empty and refers to a non-existent path **and** `hooks.install` is also missing, append a warning.

- [ ] **Step 3: Commit**

```bash
git add internal/providersupport/
git commit -m "feat(loader): warn when runner binary is absent and install hook is missing"
```

---

# PHASE 6: `mgtt provider validate`

## Task 6.1: Static validation

**Files:**
- Create: `internal/providersupport/validate/validate.go`
- Create: `internal/providersupport/validate/validate_test.go`
- Create: `internal/cli/provider_validate.go`
- Create: `internal/cli/provider_validate_test.go`
- Modify: `internal/cli/root.go`

- [ ] **Step 1: Define validation shape**

`internal/providersupport/validate/validate.go`:

```go
// Package validate runs correctness checks on a loaded provider.
package validate

import (
	"github.com/mgt-tool/mgtt/internal/providersupport"
)

type Report struct {
	Passed    []string
	Warnings  []string
	Failures  []string
}

func (r Report) OK() bool { return len(r.Failures) == 0 }

func Static(p *providersupport.Provider) Report {
	var r Report
	// Check meta
	if p.Meta.Name == "" { r.Failures = append(r.Failures, "meta.name is empty") }
	if p.Meta.Version == "" { r.Failures = append(r.Failures, "meta.version is empty") }
	if p.Meta.Command == "" { r.Failures = append(r.Failures, "meta.command is empty") }

	// Every declared fact has a probe.cmd
	for _, typ := range p.Types {
		for factName, f := range typ.Facts {
			if f.Probe.Cmd == "" {
				r.Failures = append(r.Failures, typ.Name+"/"+factName+": probe.cmd empty")
			}
			if f.Probe.Parse == "" {
				r.Warnings = append(r.Warnings, typ.Name+"/"+factName+": probe.parse empty (defaults to string)")
			}
		}
	}

	// default_active_state must be one of the declared states
	for _, typ := range p.Types {
		if typ.DefaultActiveState != "" {
			if _, ok := typ.States[typ.DefaultActiveState]; !ok {
				r.Failures = append(r.Failures,
					typ.Name+": default_active_state "+typ.DefaultActiveState+" not in states")
			}
		}
	}

	// healthy conditions must reference declared facts
	// (reuse existing expr walker — pseudo-code; real impl uses expr.Walk)
	// ...

	if len(r.Failures) == 0 && len(r.Warnings) == 0 {
		r.Passed = append(r.Passed, "static checks: ok")
	}
	return r
}
```

- [ ] **Step 2: Write tests**

One test per failure condition above + one happy-path test.

- [ ] **Step 3: Write CLI command**

`internal/cli/provider_validate.go`:

```go
package cli

import (
	"fmt"
	"github.com/mgt-tool/mgtt/internal/providersupport/validate"
)

func providerValidate(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "usage: mgtt provider validate <name> [--live]")
		return 1
	}
	p, err := loadProvider(args[0])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	rep := validate.Static(p)
	for _, w := range rep.Warnings { fmt.Fprintln(stdout, "WARN  "+w) }
	for _, f := range rep.Failures { fmt.Fprintln(stdout, "FAIL  "+f) }
	for _, p := range rep.Passed   { fmt.Fprintln(stdout, "PASS  "+p) }
	if !rep.OK() { return 1 }
	return 0
}
```

- [ ] **Step 4: Register in root.go**

- [ ] **Step 5: Run tests — expect pass**

- [ ] **Step 6: Commit**

```bash
git add internal/providersupport/validate/ internal/cli/provider_validate.go internal/cli/provider_validate_test.go internal/cli/root.go
git commit -m "feat(cli): mgtt provider validate — static correctness checks"
```

**I5 — CI policy:** `mgtt provider validate` (static) is safe to run in core CI against fixture providers in testdata. `mgtt provider validate --live` requires a real backend and MUST NOT be wired into core CI — each provider's own CI owns that invocation against its own test setup. Document this explicitly in PROBE_PROTOCOL.md under "Validation."

## Task 6.2: Live validation (`--live`)

**Files:**
- Modify: `internal/providersupport/validate/validate.go`
- Modify: `internal/cli/provider_validate.go`

- [ ] **Step 1: Add Live function**

`Live(p *providersupport.Provider)` invokes the runner for each declared type+fact, records pass/fail/skip per fact based on whether the runner recognizes it (exit 1 with "unknown fact" → fail; exit 0 with any status → pass; other exit → warn).

This provides the same guarantee the provider-hardening plan's parity test offers, but **from the core side**, so it lives in CI for every provider.

- [ ] **Step 2: Wire `--live` flag in provider_validate.go**

- [ ] **Step 3: Integration test on a tiny fixture provider**

- [ ] **Step 4: Commit**

```bash
git add internal/providersupport/validate/ internal/cli/provider_validate.go
git commit -m "feat(cli): mgtt provider validate --live — exercises runner for every declared fact"
```

---

# PHASE 6.5: Read-Only Enforcement (Defense in Depth)

**Threat model.** A provider runner is arbitrary code mgtt invokes. mgtt cannot inspect what it does at the syscall level without OS sandboxing (seccomp/namespaces), which is out of scope for v0. The `auth.access.writes: none` declaration in `provider.yaml` is therefore a **contract the provider makes with operators**, not a gate mgtt enforces. Real enforcement happens at whatever authorization boundary the backend uses — this varies completely by backend (kubernetes RBAC, cloud IAM, daemon-socket permissions, API tokens, POSIX permissions, or sometimes nothing at all for authz-free backends). Core must not assume any specific model.

What core can do is make the contract **visible, auditable, and hard to violate by accident**:

## Task 6.5.1: Surface `writes` declaration in `provider inspect` and `validate`

**Files:**
- Modify: `internal/cli/provider_inspect.go` (if exists) or equivalent
- Modify: `internal/providersupport/validate/validate.go`

- [ ] **Step 1: `validate` FAILS when `auth.access.writes` is missing or not `none`.**

Rationale: if a provider omits the field or declares writes, a human must opt in. Today every listed provider declares `writes: none`. A new provider that doesn't must justify it.

```go
// In Static():
if p.Auth.Access.Writes == "" {
    r.Failures = append(r.Failures, "auth.access.writes is not declared — must be \"none\" or explicit scope")
} else if p.Auth.Access.Writes != "none" {
    r.Warnings = append(r.Warnings,
        fmt.Sprintf("auth.access.writes=%q — operators must confirm credentials match this scope", p.Auth.Access.Writes))
}
```

- [ ] **Step 2: `provider inspect` prints the declared access surface prominently.**

```
Provider: kubernetes 2.0.0
Access:
  probes:  kubectl read-only
  writes:  none   ← must match the ServiceAccount RBAC binding
```

- [ ] **Step 3: Commit**

```bash
git add internal/cli/ internal/providersupport/validate/
git commit -m "feat(validate): enforce writes declaration; surface in provider inspect"
```

## Task 6.5.2: SDK dry-run mode — **DEFERRED (S1)**

Cut from v0. No concrete operator has requested this audit tool; the plan was speculating on who would consume it. Revisit when a real request arrives. Skip the steps below — do NOT implement.

<details>
<summary>Original (deferred) content</summary>

**Files:**
- Modify: `sdk/provider/shell/client.go`

When `MGTT_DRY_RUN=1` is set in the runner's env, `shell.Client.Run` logs the full argv to stderr and returns an empty result without executing. Gives operators a "what would this provider shell out to?" audit tool.

- [ ] **Step 1: Write failing test**

```go
func TestClient_DryRun_LogsAndSkipsExec(t *testing.T) {
	var executed bool
	c := &Client{
		DryRun: true,
		Stderr: &bytes.Buffer{},
		Exec: func(ctx context.Context, args ...string) ([]byte, []byte, error) {
			executed = true
			return nil, nil, nil
		},
	}
	_, err := c.Run(context.Background(), "get", "deploy")
	if err != nil { t.Fatal(err) }
	if executed { t.Fatal("dry-run should skip exec") }
	// stderr should contain the argv
}
```

- [ ] **Step 2: Add `DryRun bool` + `Stderr io.Writer` fields; honor env in `New()`.**

- [ ] **Step 3: Commit**

```bash
git add sdk/provider/shell/
git commit -m "feat(sdk/shell): MGTT_DRY_RUN mode — log argv, skip exec"
```
</details>

## Task 6.5.3: Generic credential-guidance convention

**Context.** Authorization models vary wildly across backends: kubernetes has RBAC (ClusterRole/RoleBinding), AWS has IAM policies, Docker relies on daemon-socket permissions or docker-contexts, Prometheus uses bearer tokens with scoped rules, a filesystem provider is just POSIX permissions, an HTTP-only provider may have no authz at all. Core must not privilege any of these.

**Core's stance:** core specifies the *field* (`auth.access.writes`) and offers a *place* for providers to document backend-specific least-privilege guidance — but never names the mechanism.

**Files:**
- Document convention in `sdk/provider/README.md` and `docs/PROBE_PROTOCOL.md`
- Add soft check in `internal/providersupport/validate/validate.go`

- [ ] **Step 1: Document in `sdk/provider/README.md`**

```markdown
## Declaring read-only access

Every provider MUST declare its access surface in `provider.yaml`:

    auth:
      access:
        probes: <human-readable description of what the provider reads>
        writes: none                # REQUIRED; mgtt validate fails otherwise

The `writes: none` declaration is a contract the provider makes with operators. mgtt cannot enforce it directly — actual enforcement lives in whatever authorization layer the backend uses (cluster RBAC, cloud IAM, POSIX permissions, API tokens, or nothing if the backend has no authz).

Providers SHOULD ship a `CREDENTIALS.md` next to `provider.yaml` describing how an operator provisions minimum-privilege credentials for this provider's backend. The file's *format* and *content* are provider-specific:

- A kubernetes provider may include ClusterRole YAML with only `get`/`list`/`watch` verbs.
- An AWS provider may include an IAM policy JSON scoped to `Describe*`/`List*`/`Get*`.
- A docker provider may describe how to bind a read-only docker context or socket.
- A provider against an authz-free backend may simply state "no credentials required; provider reads public endpoints".

Core does not inspect this file — its existence is the signal that the provider author has thought about least privilege.
```

- [ ] **Step 2: Do NOT add a validate warning (S2).**

A warning that fires for every provider not shipping the file is noise. Documentation of the convention in `sdk/provider/README.md` is sufficient. Skip.

- [ ] **Step 3: Commit**

```bash
git add sdk/provider/README.md docs/PROBE_PROTOCOL.md internal/providersupport/validate/
git commit -m "docs: require writes declaration; recommend CREDENTIALS.md (backend-agnostic)"
```

## Task 6.5.4: Future: OS sandboxing (deferred)

Document in the plan's non-goals that **true enforcement** would require running the provider under seccomp-bpf (Linux) or sandbox-exec (macOS) with network namespaces scoped to the backend API and a read-only rootfs. This is a multi-week effort, platform-specific, and fights tooling assumptions (kubectl writes to `~/.kube/cache`). **Not v0 scope** — the plan explicitly rejects this in favor of the declarative + operator-RBAC model above.

Add to the "Non-goals" section at the top of this plan:

> - **OS-level sandboxing of provider runners** — relying on operator-controlled credentials (RBAC/IAM) is the v0 enforcement model; kernel sandboxing is a v1 consideration.

---

# PHASE 7: Documentation Updates

## Task 7.1: Update providers/README.md

**Files:**
- Modify: `providers/README.md` (if exists) or `docs/providers.md`

- [ ] **Step 1: Point to SDK + PROBE_PROTOCOL.md**

Rewrite the authoring section to recommend `sdk/provider`. Keep the "advanced: raw protocol" section pointing at `docs/PROBE_PROTOCOL.md` for providers written in other languages.

- [ ] **Step 2: Commit**

```bash
git add providers/README.md
git commit -m "docs: recommend sdk/provider for Go providers; keep raw protocol docs for others"
```

## Task 7.1b: SDK importability release gate (C4)

Before tagging v0.0.7, the SDK must be externally consumable. The scratch-directory test is promoted from a sweep checklist item to a release gate.

- [ ] **Step 1: Verify no `internal/` leakage into `sdk/`**

```bash
grep -rn 'github.com/mgt-tool/mgtt/internal/' sdk/
```

Expected: zero lines. Any hit means an external consumer cannot `go get` the SDK.

- [ ] **Step 2: Scratch-directory import test**

```bash
TMP=$(mktemp -d)
pushd "$TMP"
go mod init scratch
# Point at a local replace during pre-release; at release time, use the tag.
go mod edit -replace github.com/mgt-tool/mgtt=/root/docs/projects/mgtt
cat > main.go <<'EOF'
package main

import (
    "context"
    "github.com/mgt-tool/mgtt/sdk/provider"
)

func main() {
    r := provider.NewRegistry()
    r.Register("foo", map[string]provider.ProbeFn{
        "bar": func(ctx context.Context, req provider.Request) (provider.Result, error) {
            return provider.IntResult(42), nil
        },
    })
    provider.Main(r)
}
EOF
go mod tidy
go build -o /dev/null .
popd
rm -rf "$TMP"
```

Expected: zero errors.

- [ ] **Step 3: Post-tag verification**

After `git push origin v0.0.7`, repeat Step 2 without the `replace` directive, using `go get github.com/mgt-tool/mgtt/sdk/provider@v0.0.7`. Expected: zero errors. If this fails, yank the tag and investigate.

## Task 7.2: Release v1.1.0

**Files:**
- Modify: `VERSION`

- [ ] **Step 1: Bump**

```
0.0.7
```

(Or jump to 0.1.0 if the `Status` field change counts as a minor bump under mgtt's 0.x semantics. Align with `internal/providersupport/version.go:MgttVersion` — they should match.)

- [ ] **Step 2: Commit, tag, push**

```bash
git add VERSION
git commit -m "release: v0.0.7 — probe protocol v1.1, provider SDK, validate command"
git tag v0.0.7
git push origin main --tags
```

---

# PHASE 8: Cross-Cutting Sweep

- [ ] **Layering invariant:** grep for backend-specific vocabulary in `internal/`:

  ```bash
  # Words that must NOT appear outside testdata/, docs, and string-literal error messages sourced from providers:
  grep -rni --include='*.go' --exclude-dir=testdata -E '\b(kubectl|kubernetes|aws|docker|namespace|cluster|region|pod|deployment|service|rbac|iam)\b' internal/
  ```
  
  Every remaining hit must be (a) inside an error message passing through an opaque stderr, or (b) a comment citing this file as rationale. Any true hardcoded name means a provider's vocabulary has leaked into core — fix by moving to the provider or making it a generic mechanism.

- [ ] **Interface-only dispatch:** `grep -n 'map\[string\]\*' internal/providersupport/probe/*.go` should return zero lines. All provider dispatch uses the `Executor` interface, never a concrete type.

- [ ] **Back-compat check:** existing fixture files without `status:` load as "ok". Run the full engine test suite: `go test ./...`.
- [ ] **No new runtime deps:** `go list -m all` unchanged except version bumps.
- [ ] **Debug output never on stdout:** `grep -rn "os.Stdout" internal/providersupport/probe/` returns zero lines outside of result encoding paths.
- [ ] **Protocol documented in one place only:** `docs/PROBE_PROTOCOL.md` is the canonical source; `providers/README.md` references it, `sdk/provider/README.md` references it, no other place duplicates it.
- [ ] **SDK consumable from outside:** in a scratch directory, `go mod init test && go get github.com/mgt-tool/mgtt/sdk/provider@v0.0.7 && go build` succeeds on a 10-line main.go.
- [ ] **Validate command runs green on registry.yaml providers:** for each listed provider, `mgtt provider install <name> && mgtt provider validate <name>` exits 0.
- [ ] **Error message quality (M3):** exercise each sentinel path manually (`ErrEnv`, `ErrForbidden`, `ErrTransient`, `ErrProtocol`, `ErrUsage`, `ErrUnknown`). For each, the operator-facing message MUST be ≤ 80 characters, include the resource name or context, and suggest a next action (e.g. "Forbidden: cannot get pods in ns=foo — check ServiceAccount RBAC"). If a message reads as raw Go error chaining ("provider: forbidden: pods is forbidden: User \"x\""), rewrite it. No lazy `%w: %v` chains in operator-facing paths.

---

# Revised Provider-Hardening Plan Diff

After this core plan lands, the sibling `mgtt-provider-kubernetes/docs/superpowers/plans/2026-04-15-provider-hardening.md` should be edited as follows:

**Delete:**
- Phase 1 Tasks 1.1–1.3, 1.4 (internal/kubeclient entirely) — replaced by `sdk/provider/shell`
- Phase 1 Task 1.4 (registry) — replaced by `provider.NewRegistry()`
- Phase 1 Task 1.9 (parity_test) — replaced by `mgtt provider validate --live` in CI
- Phase 2 entirely (robustness in SDK)
- Phase 3 (doctor subcommand → validate)

**Keep (unchanged):**
- Phase 0 Tasks 0.1 (CI) — add a `mgtt provider validate --live` step
- Phase 0 Task 0.2 (VERSION + ldflags) — adjust ldflags target to `github.com/mgt-tool/mgtt/sdk/provider.Version`
- Phase 4–8 (type coverage) — same content, registrations go through `provider.NewRegistry()` instead of the provider-local registry
- Phase 10 (install hook) — unchanged
- Phase 11 (release) — unchanged
- Phase 12 (final sweep) — modify parity assertion to run `mgtt provider validate --live` instead

**Net reduction:** ~20 of ~50 tasks deleted; the rest simplified. Every future provider (aws, docker, future redis/prometheus/…) reuses the SDK and validate tooling, so the per-provider cost drops permanently.
