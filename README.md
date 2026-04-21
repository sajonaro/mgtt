![mgtt](docs/images/mgtt_full_lockup.png)

## model guided troubleshooting tool

If you build or maintain anything with more than two components, you know the drill: something stops working, you open three terminals and start guessing.

**mgtt fixes this.** You describe your system's dependencies once in a YAML model. When something breaks, a constraint engine walks the dependency graph, probes components in order of information value, and eliminates healthy branches. It always knows what to check next and why.

And before the system even exists, you can simulate failures against the model to verify the reasoning is correct — like unit tests for your architecture.

## See it in action

### Troubleshooting at 3am: `mgtt diagnose`

Alert fires. You trigger `mgtt diagnose` — from CI, a Slack slash-command, an LLM agent, or your laptop — and get a structured report back:

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

4 components probed, 2 eliminated, root cause named. Partial visibility (RBAC refusals, transient throttles) surfaces as a flag in the report rather than aborting the session.

### Simulation in CI: `mgtt simulate`

Before the system is even running, `mgtt simulate` verifies the model's reasoning against hand-authored scenarios (*"if rds goes down and api crash-loops, root cause is rds"*). Wire it into every PR to catch **model drift** before the model is needed at 3am.

```
$ mgtt simulate --all

  rds unavailable                          ✓ passed
  api crash-loop independent of rds        ✓ passed
  all components healthy                   ✓ passed

  3/3 scenarios passed
```

No running system, no credentials. Runs anywhere Go runs.

---

## Install

```bash
curl -sSL https://raw.githubusercontent.com/mgt-tool/mgtt/main/install.sh | sh
```

Or: `go install github.com/mgt-tool/mgtt/cmd/mgtt@latest` | `docker run --rm -v $(pwd):/workspace ghcr.io/mgt-tool/mgtt`

## Quick start

```bash
mgtt init                          # scaffold system.model.yaml
mgtt model validate                # check the model
mgtt provider install kubernetes   # install providers
mgtt simulate --all                # run failure scenarios (in CI)
mgtt diagnose                      # troubleshoot a live system
```

**Two moments, one model:**

| | Design time | At 3am |
|---|---|---|
| Command | `mgtt simulate` | `mgtt diagnose` |
| Facts from | Scenario YAML | Real probes |
| Driven by | CI pipeline | On-call engineer, CI job, Slack bot, or AI agent |
| Output | Pass/fail | Root cause + chain + eliminated components |

`mgtt plan` exists too — same engine, interactive press-Y mode for debugging models or teaching. Not the daily driver.

---

## Documentation

- [Quick Start](./docs/getting-started/quickstart.md) — end-to-end in five minutes
- [Blue/green storefront worked example](./docs/examples/blue-green-storefront.md) — a realistic 20-component system, five scenarios, the refinements that came from real use
- [How It Works](./docs/concepts/how-it-works.md) — the constraint engine and dependency graph
- [Model Schema](./docs/reference/model-schema.md) — every field in `system.model.yaml`
- [Documentation site](https://mgt-tool.github.io/mgtt) — full reference, provider catalog, specs

## For TLA+ users

TLA+ checks your design; mgtt checks your running system. Same idea — write the spec, let the tool do the thinking — pointed at a different problem.

## License

Dual-licensed: **core engine and CLI** under [AGPL-3.0](LICENSE); **provider SDK** (`sdk/provider/`) under [Apache-2.0](sdk/provider/LICENSE) so third-party provider authors can ship under any licence. See [`sdk/provider/NOTICE`](sdk/provider/NOTICE) for the rationale.
