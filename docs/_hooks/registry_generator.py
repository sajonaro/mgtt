"""Build-time hook: regenerate docs/reference/registry.md from upstream provider.yamls.

Wired in mkdocs.yml under `hooks:`. Runs once per build via on_pre_build.
"""

from __future__ import annotations

from pathlib import Path

import yaml

REPO_ROOT = Path(__file__).resolve().parent.parent.parent
REGISTRY_YAML = REPO_ROOT / "docs" / "registry.yaml"
REGISTRY_MD = REPO_ROOT / "docs" / "reference" / "registry.md"
CACHE_DIR = REPO_ROOT / ".cache" / "registry-generator"


def on_pre_build(config, **_kwargs):
    """mkdocs hook entry point."""
    raise NotImplementedError("wired in later tasks")


DEFAULTS = {"channel": "latest-tag", "skip_image": False}


def load_registry(stream) -> dict[str, dict]:
    data = yaml.safe_load(stream) or {}
    providers = data.get("providers") or {}
    out = {}
    for name, entry in providers.items():
        merged = {**DEFAULTS, **(entry or {})}
        if "url" not in merged:
            raise ValueError(f"registry entry {name!r}: url is required")
        out[name] = merged
    return out
