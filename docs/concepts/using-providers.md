# Using Providers

How mgtt consumes providers at runtime — install mechanics, dispatch, capability forwarding, and the operator knobs for all of it. This is the ops-angle complement to [Writing Providers](../providers/overview.md) (which is the author-angle).

## On this page

- [The lifecycle](#the-lifecycle) — discover → install → dispatch → uninstall
- [How mgtt invokes a provider](#how-mgtt-invokes-a-provider) — git binary vs docker container
- [What mgtt forwards — the capability contract](#what-mgtt-forwards-the-capability-contract)
- [Operator controls](#operator-controls) — every knob mgtt exposes
- [Auditing what's installed](#auditing-whats-installed)
- [Troubleshooting](#troubleshooting)

---

## The lifecycle

```
  mgtt provider install <name|url|--image ref>
         ↓
    ~/.mgtt/providers/<name>/                     ← provider on disk
    ├── provider.yaml
    ├── types/
    ├── bin/provider        (git install only)
    └── .mgtt-install.json  ← method, source, version, installed_at
         ↓
  mgtt plan
    engine builds a probe queue
         ↓
    for each probe, mgtt invokes the provider binary
    with a protocol envelope: `probe <component> <fact> --type T [--flag val…]`
         ↓
    provider emits `{"value": …, "raw": "…"}` on stdout
         ↓
  mgtt provider uninstall <name>
    runs hooks/uninstall.sh (git installs only), removes the directory
```

Three invariants hold across methods: every probe goes through the same protocol envelope; every install lands in the same on-disk layout; the engine doesn't know or care whether a provider is a local binary or a container.

---

## How mgtt invokes a provider

Two install methods, two dispatch paths.

### Git install → local binary

`mgtt provider install <git-url>` clones the repo, runs `hooks/install.sh` (which builds the binary), and lands the result at `~/.mgtt/providers/<name>/bin/provider`. At probe time mgtt invokes:

```bash
<install-dir>/bin/provider probe <component> <fact> --type <type> [--var val …]
```

No container, no isolation. The binary runs as the operator's process, sees the full shell environment, reads any files the operator can read. This is the "zero-friction" path — whatever credentials and tools your shell can reach, the provider can reach.

### Image install → `docker run`

`mgtt provider install --image <ref>@sha256:…` pulls the image, `docker cp`s `/provider.yaml` and `/types/` out of it into the install directory, records the digest in `.mgtt-install.json`, and stops. The binary stays in the image. At probe time mgtt invokes:

```bash
docker run --rm [--network <mode>] [<cap flags…>] <image-ref> \
  probe <component> <fact> --type <type> [--var val …]
```

The three moving pieces on that line are all declarations from the provider's `provider.yaml`, expanded by mgtt:

- `--network <mode>` comes from the top-level `network:` field (omitted when the mode is the default `bridge`).
- `<cap flags…>` is `capabilities.Apply(p.Needs)` — for each entry in `needs:`, mgtt injects the matching `-v`/`-e` flags from the capability vocabulary.
- `<image-ref>` is the pinned digest from `.mgtt-install.json`.

Image installs **isolate by default**: the container gets none of your shell environment unless a capability explicitly forwards it. That's the whole point — `mgtt plan` installing a pulled-from-GHCR binary that silently mounts `~/.ssh/` would be a trust disaster. Capabilities make the access grants explicit and reviewable.

---

## What mgtt forwards — the capability contract

Every image-installed probe runs in a container. The provider's `provider.yaml` declares what mgtt forwards:

```yaml
needs: [kubectl, aws]   # host packages / credential chains / sockets
network: host           # docker-run network mode
read_only: true         # posture (default true; set false with writes_note)
```

mgtt owns the vocabulary — the label-to-flag mapping. Each entry in `needs:` expands to bind mounts and env forwards:

| Capability | Injects into `docker run` |
|---|---|
| `kubectl` | `-v $HOME/.kube:/root/.kube:ro -e KUBECONFIG=…` |
| `aws` | `-v $HOME/.aws:/root/.aws:ro -e AWS_PROFILE -e AWS_ACCESS_KEY_ID …` |
| `docker` | `-v /var/run/docker.sock:/var/run/docker.sock` |
| `terraform` | `-v $PWD:/workspace -w /workspace -e TF_VAR_* …` |
| `gcloud` | `-v $HOME/.config/gcloud:/root/.config/gcloud:ro …` |
| `azure` | `-v $HOME/.azure:/root/.azure:ro -e ARM_* …` |

Env passthrough emits `-e KEY=VALUE` **only when `KEY` is set in the caller**. Unset keys are silently skipped so Docker doesn't consume the next positional arg. Full reference and operator-override schema: [Provider Capabilities](../reference/image-capabilities.md).

The `network:` field is separate from `needs:` because it names a runtime isolation setting, not a host resource. Valid values: `bridge` (default), `host`, `none`. Providers that need in-cluster DNS (`*.svc`) or host-local services declare `network: host`.

`read_only:` defaults to `true`. When a provider declares `read_only: false` (Terraform's `drifted` fact is the prototypical case — it refreshes state), it must include a `writes_note:` describing the side effect. `mgtt provider install` prints that note so you consent knowingly before it installs.

---

## Operator controls

Every knob mgtt exposes for how it runs providers:

### `MGTT_HOME`

Base directory for installed providers, capability overrides, registry cache, everything mgtt writes. Defaults to `$HOME/.mgtt`. Set it explicitly to isolate mgtt from your shell's state (tests, multi-tenant hosts, etc.):

```bash
export MGTT_HOME=/opt/mgtt
mgtt provider install kubernetes
# lands under /opt/mgtt/providers/kubernetes/
```

### `MGTT_IMAGE_CAPS_DENY`

Comma-separated list of capability names mgtt will refuse to inject regardless of what the provider declared. Use for locked-down environments that shouldn't share specific resources:

```bash
# Never mount the Docker socket into a provider image, ever.
export MGTT_IMAGE_CAPS_DENY=docker
mgtt plan
```

Denied caps are filtered at probe time — the `docker run` line goes without them. The probe likely fails with a sensible error ("cannot connect to docker daemon"), which is better than silently succeeding with wrong state.

### `MGTT_IMAGE_CAP_<NAME>`

Per-capability override via env var. Overrides both the built-in expansion and any file-based override. Argv is shell-split; single and double quotes group tokens.

```bash
# Non-default kubeconfig on bastion hosts.
export MGTT_IMAGE_CAP_KUBECTL="-v /etc/kubernetes/admin.conf:/root/.kube/config:ro -e KUBECONFIG=/root/.kube/config"
mgtt plan
```

Useful for CI one-liners. For anything long-lived, prefer the file-based override (below).

### `$MGTT_HOME/capabilities.yaml`

Persistent operator overrides. Replaces a built-in cap's expansion or defines a new custom cap. Works alongside optional drop-in shards under `$MGTT_HOME/capabilities.d/*.yaml`:

```yaml
# $MGTT_HOME/capabilities.yaml
capabilities:
  # Override the built-in kubectl cap for a non-default kubeconfig path.
  kubectl:
    - "-v"
    - "/etc/kubernetes/admin.conf:/root/.kube/config:ro"
    - "-e"
    - "KUBECONFIG=/root/.kube/config"

  # Define a custom cap used by an internal provider.
  tibco:
    - "-v"
    - "/etc/tibco/cert.pem:/root/cert.pem:ro"
    - "-e"
    - "TIBCO_BROKER_URL"
```

Precedence (highest wins): env-var override → operator file → built-in.

### `MGTT_PROBE_TIMEOUT`

Override the default 30-second per-probe timeout. Useful when a provider's probe is legitimately slow (`terraform plan` against a cold cloud backend is the canonical case):

```bash
export MGTT_PROBE_TIMEOUT=120s
mgtt plan --component ingress
```

### `MGTT_REGISTRY_URL`

Override the registry index for `mgtt provider install <name>` name-to-URL resolution. Accepts `https://…`, `file:///path/to/registry.yaml` (local mirror), and the sentinels `disabled` / `none` / `off` (air-gapped — bare-name installs fail with a clear error, only git URLs and `--image` refs are accepted).

---

## Auditing what's installed

`mgtt provider ls` shows every installed provider at a glance:

```
  ✓ kubernetes  v2.3.1  image  [kubectl]         Kubernetes cluster resources via kubectl
  ✓ tempo       v0.2.0  git    -                 Per-span SLO checks against Grafana Tempo
  ✓ terraform   v0.1.0  image  [terraform, aws]  Terraform-managed infrastructure
```

Columns: method (git|image), declared capabilities (`-` for none), description.

`mgtt provider inspect <name>` shows the full contract for one provider — posture, writes-note (when not read-only), needs, network, types, state machines, failure modes. Use it before installing a provider you don't recognize.

The [public registry](../reference/registry.md) lists every mgt-tool provider with the same metadata — capabilities and network columns so you can audit *before* installing.

---

## Troubleshooting

### "The probe can't reach my cluster"

Image-installed Kubernetes probes need `network: host` to reach in-cluster DNS (`kubernetes.default.svc`) or private API server IPs. Bridge-mode containers can't resolve those. Verify the provider's `provider.yaml` declares `network: host`:

```bash
cat $MGTT_HOME/providers/kubernetes/provider.yaml | grep network
```

If it's missing or set to `bridge`, the provider's author hasn't opted in. For kubernetes/terraform/tempo/quickwit this is a bug in the provider; file it against the provider repo.

### "A capability I declared isn't being applied"

Three things to check, in order:

1. **`MGTT_IMAGE_CAPS_DENY`** — is the env var set and does it list the cap? mgtt emits a stderr warning when it skips a declared cap because of DENY.
2. **Provider was installed with git, not image** — caps only apply to image installs. `mgtt provider ls` shows the method column.
3. **Capability doesn't exist** — `mgtt provider install` rejects unknown caps at install time, but a stale `.mgtt-install.json` can survive if you bump the provider's `needs:` list after install. Reinstall the provider.

### "I want to refuse the Docker socket"

```bash
export MGTT_IMAGE_CAPS_DENY=docker
```

Any image-installed provider that declares `needs: [docker]` will now run without the socket mount. The probe will fail with `cannot connect to docker daemon` — a legible error, not silent wrong state. Relevant mainly for `mgtt-provider-docker`.

### "An AWS env var isn't being forwarded"

`-e KEY=VALUE` is emitted only when `KEY` is set (non-empty) in the caller's environment. If you're using instance-profile creds (no `AWS_ACCESS_KEY_ID` set), only the profile-dir mount fires — the `-e` flags are correctly absent. That's by design: a bare `-e KEY` would make `docker run` consume the next positional arg.

If you expected a variable to flow through and it isn't, verify it's exported in the shell mgtt runs from:

```bash
env | grep AWS_
```

### "The install failed with 'unknown capability'"

The provider's `provider.yaml` declares a label mgtt doesn't know. Two options:

1. The provider author invented a label — file an issue; built-in additions need a mgtt PR.
2. You're running a provider that expects a custom operator capability — define it in `$MGTT_HOME/capabilities.yaml` before installing.

---

## See also

- [Provider Install Methods](provider-install-methods.md) — git vs image, digest pinning, what lands on disk
- [Provider Capabilities](../reference/image-capabilities.md) — full capability vocabulary and override schema
- [Writing Providers](../providers/overview.md) — the author-angle: authoring `provider.yaml`, binary protocol, install hooks
- [Configuration](../reference/configuration.md) — every `MGTT_*` env var in one place
