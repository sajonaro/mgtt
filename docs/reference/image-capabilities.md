# Image Capabilities

`mgtt provider install --image` pulls a provider image; `mgtt plan` runs probes against it via `docker run`. This page is the reference for the **capability vocabulary** that bridges the two — the named labels a provider declares so that `docker run` is invoked with the right bind mounts and env forwards.

## Declaring capabilities

In your `provider.yaml`:

```yaml
image:
  needs: [kubectl, network]
```

`image.needs` is optional. Providers that don't talk to host resources (tempo, quickwit — they configure their URL via `vars:`) can omit it entirely.

## Built-in vocabulary

| Capability | What mgtt injects |
|---|---|
| `network` | `--network host` |
| `kubectl` | `-v $HOME/.kube:/root/.kube:ro`; `-e KUBECONFIG=…` (when set) |
| `aws` | `-v $HOME/.aws:/root/.aws:ro`; `-e` for each of `AWS_PROFILE`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`, `AWS_REGION`, `AWS_DEFAULT_REGION` (when set) |
| `docker` | `-v /var/run/docker.sock:/var/run/docker.sock` |
| `terraform` | `-v $PWD:/workspace -w /workspace`; `-e TF_CLI_CONFIG_FILE` and every `TF_VAR_*` set in the caller |
| `gcloud` | `-v $HOME/.config/gcloud:/root/.config/gcloud:ro`; `-e GOOGLE_APPLICATION_CREDENTIALS` and every `CLOUDSDK_*` set |
| `azure` | `-v $HOME/.azure:/root/.azure:ro`; `-e` for `ARM_CLIENT_ID`, `ARM_CLIENT_SECRET`, `ARM_TENANT_ID`, `ARM_SUBSCRIPTION_ID` (when set) |

Env passthrough emits `-e KEY=VALUE` only when `KEY` is set in the caller. Unset keys are silently skipped so Docker never consumes the next positional arg.

## Operator overrides

Drop a file at `$MGTT_HOME/capabilities.yaml`, or shards under `$MGTT_HOME/capabilities.d/*.yaml`:

```yaml
capabilities:
  # Override built-in kubectl for a non-default kubeconfig
  kubectl:
    - "-v"
    - "/etc/kubernetes/admin.conf:/root/.kube/config:ro"
    - "-e"
    - "KUBECONFIG=/root/.kube/config"

  # Define a custom capability used by an internal provider
  tibco:
    - "-v"
    - "/etc/tibco/cert.pem:/root/cert.pem:ro"
    - "-e"
    - "TIBCO_BROKER_URL"
```

Precedence (highest wins): `MGTT_IMAGE_CAP_<NAME>` env var → operator file → built-in.

**Env-var one-liner** (for CI without a file):

```
MGTT_IMAGE_CAP_KUBECTL="-v /etc/k.conf:/root/.kube/config:ro -e KUBECONFIG=/root/.kube/config"
```

Argv is shell-split; single and double quotes group tokens.

## Opt-out

```
MGTT_IMAGE_CAPS_DENY=docker,aws
```

Comma-separated list. mgtt refuses to inject these capabilities regardless of provider declaration. The probe still runs — without the forwards — and likely fails with an honest error, not silent wrong state.

## Validation

`mgtt provider validate` and `mgtt provider install --image` both check `image.needs` against the merged vocabulary (built-ins ∪ operator file ∪ env overrides). Unknown caps fail with a message naming the offending label and the known-names list. Shell-fallback providers (no `meta.command`) cannot declare capabilities — there's no binary in the image to inject them into.
