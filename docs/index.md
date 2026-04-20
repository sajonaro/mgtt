![mgtt](images/mgtt_full_lockup.png)

If you build or maintain anything with more than two components — a web app with a frontend, an API, and a database; a set of microservices behind a load balancer; a data pipeline with queues, workers, and storage — you know the drill:

<div class="scenario" markdown>

**Something stops working.**

- You check the logs. *Nothing obvious.*
- You check the database. *Looks fine.*
- You check the API. *Restarting. Why?*
- You check the config.
- You check the deploy history.
- You ask the person who wrote it. *They're asleep.*
- You open three terminals and start guessing.

</div>

<div class="problem" markdown>

**The core problem.** Troubleshooting distributed systems is slow, unstructured, and depends entirely on whoever happens to know the system.

- No map.
- No systematic narrowing.
- No way to know what you've already ruled out.

</div>

<div class="approach" markdown>

**mgtt fixes this.**

- **Describe once.** Your system's dependencies in a single YAML model.
- **Walk the graph.** When something breaks, a constraint engine walks the dependency graph, probing components in order of information value and eliminating healthy branches.
- **Know what's next.** Always — and why.

</div>

Press Y at each step yourself, or let an AI agent drive the same loop autonomously through the same interface. **The engine reasons; whoever's on call executes.**

## See it in action

### Simulation: reason about failures without running the system

Write a scenario — a tiny YAML assertion like *"if rds goes down and api crash-loops, the engine should blame rds, not api"* — and `mgtt simulate` checks that the engine concludes the same thing. No live system, no credentials, no cluster access. Runs anywhere Go runs.

What you get once it's wired into CI:

- **Model drift detection** — when the real system evolves (new services, renamed components, changed dependencies), a stale model silently drifts away from reality. Simulate on every PR and a failing scenario tells you *before* the model is needed at 3am.
- **Architecture unit tests** — each scenario is a declarative assertion. Refactor the model, break a conclusion, the suite fails. Safe renames, safe dependency moves.
- **Design-time validation** — write the model before the system exists, reason about whose-depends-on-whom, find the holes before you build them. The engine treats your design as executable logic.
- **Regression harness** — the next time a real incident happens, encode it as a scenario. The engine must now identify that chain forever. Your postmortems become tests.

```
$ mgtt simulate --all

  rds unavailable                          ✓ passed
  api crash-loop independent of rds        ✓ passed
  frontend crash-looping, api healthy      ✓ passed
  all components healthy                   ✓ passed

  4/4 scenarios passed
```

[Full simulation walkthrough](concepts/simulation.md)

### Troubleshooting: root cause in 6 probes

Monday 3am. Alert fires. You run `mgtt plan` and press Y:

```
$ mgtt plan

  -> probe nginx upstream_count
     cost: low | kubectl read-only

  ✓ nginx.upstream_count = 0   ✗ unhealthy

  -> probe api restart_count
     cost: low

  ✓ api.restart_count = 47   ✗ unhealthy

  -> probe rds available
     cost: low | AWS API read-only

  ✓ rds.available = true   ✓ healthy       ← eliminated

  -> probe frontend ready_replicas
     cost: low | kubectl read-only

  ✓ frontend.ready_replicas = 2   ✓ healthy  ← eliminated

  Root cause: api
  Path:       nginx <- api
  State:      degraded
  Eliminated: frontend, rds
```

> **4 components probed. 2 eliminated. Root cause found.** You didn't need to know the system — the model knew it for you. An AI agent could run the same loop autonomously.

[Full troubleshooting walkthrough](concepts/troubleshooting.md)

### Autopilot: `mgtt diagnose`

Same engine, no prompts. Hand it a probe budget and get a final report:

```
$ mgtt diagnose --suspect api --max-probes 10

  ▶ probe nginx upstream_count       ✗ unhealthy
  ▶ probe api ready_replicas         ✗ unhealthy
  ▶ probe rds available              ✓ healthy  ← eliminated
  ▶ probe frontend ready_replicas    ✓ healthy  ← eliminated

  Root cause: api.degraded
  Chain:      nginx ← api
  Probes run: 4/10
```

Failure chains are pre-enumerated into a committed `scenarios.yaml` at design time, so diagnose eliminates whole branches before running a probe.

[`mgtt diagnose` reference](concepts/troubleshooting.md#autopilot-mode-mgtt-diagnose) | [scenarios.yaml](reference/scenarios-yaml.md)

---

## What mgtt gives you

One model, three moments:

- **Model once** — describe components, dependencies, and what "healthy" means in YAML.
- **Simulate in CI** — inject synthetic failures; assert the engine reasons correctly; catch model gaps before production.
- **Troubleshoot at 3am** — press Y (`mgtt plan`) or hand the loop over (`mgtt diagnose`); the engine picks the most informative probe at every step.

|             | Design time     | At 3am (interactive)         | At 3am (autopilot)          |
|-------------|-----------------|------------------------------|-----------------------------|
| Command     | `mgtt simulate` | `mgtt plan`                  | `mgtt diagnose`             |
| Facts from  | scenario YAML   | real probes + Y/n            | real probes, no prompts     |
| Driven by   | CI pipeline     | SRE                          | AI agent or unattended run  |
| Output      | pass/fail       | guided root cause            | final report + chain        |

## Get started

- [Quick Start](getting-started/quickstart.md) — complete end-to-end example: model, scenarios, simulate
- [Install](getting-started/install.md) — one-liner, Go, Docker, from source

## Learn

- [How It Works](concepts/how-it-works.md) — the constraint engine and dependency graph
- [Simulation walkthrough](concepts/simulation.md) — design-time model validation
- [Troubleshooting walkthrough](concepts/troubleshooting.md) — runtime incident response

## Working with Providers

- [Using Providers](concepts/using-providers.md) — how mgtt invokes providers at probe time
- [Install Methods](concepts/provider-install-methods.md) — git build vs. pre-built Docker image
- [Names and Versions](concepts/provider-fqn-and-versions.md) — FQN + version constraint resolution
- [Provider Capabilities](reference/image-capabilities.md) — `needs:` vocabulary and operator overrides

## Provider Registry

- [Official and community providers](reference/registry.md) — browse what's available, copy the install line

## Reference

- [Model Schema](reference/model-schema.md) — every field in `system.model.yaml`
- [Scenario Schema](reference/scenario-schema.md) — hand-authored `scenarios/*.yaml` for `mgtt simulate`
- [`scenarios.yaml`](reference/scenarios-yaml.md) — the generated sidecar `mgtt diagnose` consumes
- [Type Catalog](reference/type-catalog.md) — all provider types, facts, states, and failure modes
- [CLI Reference](reference/cli.md) — every command
- [Full Specification](reference/spec.md) — the v1.0 spec

## Extend

- [Writing Providers](providers/overview.md) — teach mgtt about your technology
