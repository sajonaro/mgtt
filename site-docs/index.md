# MGTT — Model Guided Troubleshooting Tool

Troubleshooting distributed systems is usually a battle of wits. `mgtt` replaces that with a structured loop: describe your system once, observe facts as you go, let the constraint engine tell you what to check next.

## Two Modes of Operation

### Simulation (design time)

Write failure scenarios before deploying. Validate that your model reasons correctly in CI.

```
$ mgtt simulate --all

  rds unavailable                          ✓ passed
  api crash-loop independent of rds        ✓ passed
  frontend crash-looping, api healthy      ✓ passed
  all components healthy                   ✓ passed

  4/4 scenarios passed
```

### Troubleshooting (runtime)

When something breaks, `mgtt plan` walks you to the root cause:

```
$ mgtt plan

  -> probe api ready_replicas
     cost: low | kubectl read-only

  ✓ api.ready_replicas = 0   ✗ unhealthy

  -> probe rds available
     cost: low | AWS API read-only

  ✓ rds.available = true   ✓ healthy

  Root cause: api
  State:      degraded
  Eliminated: frontend, rds
```

## Quick Links

- [Quick Start](getting-started/quickstart.md) — model, validate, simulate in 5 minutes
- [Writing Providers](providers/overview.md) — teach mgtt about your technology
- [CLI Reference](reference/cli.md) — every command
- [Specification](reference/spec.md) — the full v1.0 spec
- [Provider Registry](reference/registry.md) — community providers
