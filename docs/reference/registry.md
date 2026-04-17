# Provider Registry

Community-maintained providers for mgtt.

The **single source of truth** is [`docs/registry.yaml`](https://github.com/mgt-tool/mgtt/blob/main/docs/registry.yaml) in the mgtt repo. The table below is a convenience snapshot — if it's out of date, the YAML file wins.

`mgtt provider install <name>` fetches the registry index from GitHub Pages to resolve provider names to git URLs:

```
https://mgt-tool.github.io/mgtt/registry.yaml
```

## Current providers

<!-- To update: read docs/registry.yaml and mirror here. -->

| Provider | Description | Capabilities | Tags | Install |
|----------|-------------|--------------|------|---------|
| [kubernetes](https://github.com/mgt-tool/mgtt-provider-kubernetes) | Kubernetes cluster resources via kubectl | `kubectl`, `network` | workloads, networking, scaling, storage, cluster, prerequisites, rbac, webhooks, extensibility | `mgtt provider install kubernetes` |
| [aws](https://github.com/mgt-tool/mgtt-provider-aws) | AWS managed services via aws-cli | `aws`, `network` | databases, compute, messaging, storage | `mgtt provider install aws` |
| [docker](https://github.com/sajonaro/mgtt-provider-docker) | Docker containers via docker inspect | `docker` | containers | `mgtt provider install docker` |
| [terraform](https://github.com/mgt-tool/mgtt-provider-terraform) | Terraform-managed infrastructure — state health, drift detection, config-vs-reality reasoning | `terraform`, `aws`, `network` | iac, terraform, drift | `mgtt provider install terraform` |
| [tempo](https://github.com/mgt-tool/mgtt-provider-tempo) | Per-span SLO checks against Grafana Tempo — current_p99, breach_duration, error_rate | `network` | tracing, otel, grafana, slo | `mgtt provider install tempo` |
| [quickwit](https://github.com/mgt-tool/mgtt-provider-quickwit) | Cross-span tracing checks against Quickwit — transaction_flow, async_hop, consumer_health | `network` | tracing, otel, quickwit, slo, async | `mgtt provider install quickwit` |

The **Capabilities** column lists what each provider declares in its `provider.yaml` `image.needs` block — the host resources mgtt forwards when you install via `mgtt provider install --image`. See [Image Capabilities](./image-capabilities.md) for what each label expands to (bind mounts, env forwards, `--network host`) and how operators override or refuse them. Git-installed providers don't go through this layer; capabilities only affect the `--image` path.

Run `mgtt provider inspect <name>` after install to see the full type catalog.

## Publishing your provider

1. Create a git repo with the [provider structure](../providers/overview.md).
2. Ensure `mgtt provider install <your-repo-url>` works.
3. Open a PR adding your entry to [`docs/registry.yaml`](https://github.com/mgt-tool/mgtt/blob/main/docs/registry.yaml) — the table above and the programmatic index are both derived from that one file.
