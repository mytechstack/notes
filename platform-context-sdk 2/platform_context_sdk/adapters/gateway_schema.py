"""
Gateway config schema + per-service config adapter.

Production note: swap `_load_yaml` for a client call to wherever these
configs actually live (a config-management service, a git-backed API) —
keep the block-extraction and precedent-finding logic, which is the real
context-engineering work regardless of where the bytes come from.
"""
import os
import yaml

from . import REPO_ROOT

SCHEMA_PATH = os.path.join(REPO_ROOT, "platform", "schemas", "gateway-config.schema.yaml")
SERVICES_DIR = os.path.join(REPO_ROOT, "services")


def _load_yaml(path: str) -> dict:
    if not os.path.exists(path):
        return {}
    with open(path) as f:
        return yaml.safe_load(f) or {}


def get_schema_block(key: str) -> str:
    """SELECTION + COMPRESSION: just the one schema block, not the full spec."""
    schema = _load_yaml(SCHEMA_PATH)
    block = {key: schema.get(key, {})}
    return yaml.dump(block, sort_keys=False).strip()


def get_service_config_block(service: str, key: str) -> str | None:
    """
    SELECTION: just one block from one service's gateway.yaml.
    Returns None if the service has no config for that key (e.g. target
    file before the feature is added) — callers render this explicitly
    rather than silently omitting it, so the model knows what's missing
    versus what wasn't retrieved.
    """
    path = os.path.join(SERVICES_DIR, service, "gateway.yaml")
    config = _load_yaml(path)
    if key not in config:
        return None
    return yaml.dump({key: config[key]}, sort_keys=False).strip()


def find_precedent(feature_key: str, exclude_service: str) -> str | None:
    """
    SELECTION: find one other service that already has this config block,
    to use as a concrete precedent. Returns the first match — in
    production this would rank by recency, similarity of traffic
    pattern, or review status, not just "first found".
    """
    for service in sorted(os.listdir(SERVICES_DIR)):
        if service == exclude_service:
            continue
        path = os.path.join(SERVICES_DIR, service, "gateway.yaml")
        config = _load_yaml(path)
        if feature_key in config:
            return service
    return None


def get_target_file_raw(service: str) -> str:
    path = os.path.join(SERVICES_DIR, service, "gateway.yaml")
    if not os.path.exists(path):
        return "(file does not exist yet)"
    with open(path) as f:
        return f.read().strip()
