# Multi-File Models

One system can be described by more than one model file. Each file answers one question well; when you find yourself stretching a single file to cover two unrelated concerns, that's the cue to split.

## On this page

- [When to split](#when-to-split)
- [When NOT to split](#when-not-to-split)
- [Naming pattern](#naming-pattern)
- [File header convention](#file-header-convention)
- [Switching between models](#switching-between-models)
- [Worked example: Tempo provider](#worked-example-tempo-provider)

---

## When to split

Split into a separate file when the new model answers a **different question** from the existing one. Three signals that you've crossed that line:

| Signal | Single file | Separate file |
|---|---|---|
| **Operational moment** the model is right for | "All the time" — steady state | A specific window: deploy, migration, incident-response, scheduled batch |
| **Components in scope** | The same set of components | A different slice (one team's services, one tier, one region) |
| **Numeric SLOs** | One contract per component, evaluated continuously | Tighter or looser bounds *for the same components* during a defined event |

Each of those is a different question. Stretching one file to cover two questions makes both answers harder to read — and surprises operators who don't know which numbers are "in force" right now.

The cleanest decomposition lines:

1. **Steady-state vs deployment moment.** Steady-state SLOs aren't right during a blue/green switch — the new color is half-warm, the old color is half-going-away, and the contracts both should hold are not the same as the contracts each holds in isolation.
2. **Run-time vs migration window.** Schema migrations, traffic shifts, region failovers — finite operations with their own invariants ("no row count drops > 0.1%", "p99 stays < 5s during shift") that aren't applicable before or after.
3. **Tier or scope.** Platform-wide invariants live in one file; one team's service-level invariants live in their own. Both can co-exist because operators use them at different times.

## When NOT to split

Don't split for any of these — use one file with parameterization instead:

- **Different environments** (staging vs prod). Same shape, different numbers — express via vars or per-env override files, not duplicated component definitions.
- **One-var differences.** If file B is "file A but with `target_max: 2s` instead of `1s`," it's a var override, not a separate model.
- **Future-tense scenarios you might want.** Don't pre-create files for cases that don't exist yet. Add a model when there's an actual moment to invoke it for.

## Naming pattern

Filenames read as noun phrases. Pattern:

```
<system>.model.yaml                          # the canonical baseline
<system>-<moment>.model.yaml                 # a specific operational window
<system>-<scope>.model.yaml                  # a narrowed view (one team, tier, region)
```

Concrete:

```
magento-platform.model.yaml                  # baseline — always loaded
magento-blue-green-canary.model.yaml         # canary — loaded right after a deploy
magento-payment-team.model.yaml              # narrowed scope — what payment cares about
magento-region-failover.model.yaml           # one-off — loaded during DR drills
```

The baseline file owns the unmodified system name; specialized files extend it with a hyphenated qualifier.

## File header convention

Every model file should open with three things, in this order:

1. **What question this file answers** — one sentence, in plain operator-language.
2. **When to invoke it** — what operational moment makes this model the right contract.
3. **Companion models** — a one-line pointer to other files in the same directory, so a reader can navigate the family.

Example header:

```yaml
# A real Magento storefront, observed during the *moment* of a blue/green
# deploy — when steady-state SLOs aren't the right contract because the
# fleet is half-warm and half-going-away.
#
# Invoke this model right after the selector flip, as the "are we OK to
# keep going?" gate before scaling the old color down.
#
# Companion models:
#   • magento-platform.model.yaml — steady-state SLOs (always loaded)
```

The header carries the methodology in-file — readers don't need to consult docs to know what they're looking at.

## Switching between models

`mgtt plan` and `mgtt simulate` read one model at a time, selected via `MGTT_MODEL`:

```bash
# Steady state — what runs continuously
export MGTT_MODEL=models/magento-platform.model.yaml
mgtt plan

# After a deploy switch — the canary contract
export MGTT_MODEL=models/magento-blue-green-canary.model.yaml
mgtt plan
```

In CI / deploy pipelines, the operational moment determines which `MGTT_MODEL` is in scope:

```bash
# Post-deploy hook — wait 60s for spans to land, then run the canary
sleep 60
MGTT_MODEL=models/magento-blue-green-canary.model.yaml mgtt plan --fail-on-breach
```

Models don't compose at load time today — each file is read in isolation. Cross-file references are deliberate: a canary model's `depends_on` can name a `kubernetes.service` component without redefining the entire infra layer, because that component already exists in the running cluster — the model is a *view* over reality, not a definition of it.

## Worked example: Tempo provider

The [`mgtt-provider-tempo`](https://github.com/mgt-tool/mgtt-provider-tempo) repo ships two example models for the same Magento storefront, demonstrating the split:

- [`magento-platform.model.yaml`](https://github.com/mgt-tool/mgtt-provider-tempo/blob/main/examples/magento-platform.model.yaml) — **steady-state SLOs.** Four customer-facing operations (catalog browse, add to cart, checkout init, search) each held to a three-number contract.
- [`magento-blue-green-canary.model.yaml`](https://github.com/mgt-tool/mgtt-provider-tempo/blob/main/examples/magento-blue-green-canary.model.yaml) — **the deployment moment.** A single canary SLO using `span_filter` to scope to the just-promoted color, with depends_on across `kubernetes.*` and `tracing.*` so a stuck switch surfaces from two angles.

Both files are valid models for the same system. Operators load whichever matches the moment they're in.
