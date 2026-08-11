"""
Adapters — one module per source system.

Each adapter here reads from a local fixture directory to keep this SDK
runnable without real infrastructure. In production, swap the file reads
for real calls: the ADR index becomes a call to your doc-search API, the
service catalog becomes a call to your CDN/registry, etc. The function
signatures are the contract application teams depend on — keep those
stable even as the implementation underneath changes.
"""
import os

# Points at the sample repo used by tests/examples. In production this
# would be replaced by real API clients (an ADR search endpoint, a
# service-catalog client, a deploy-pipeline client) — not a filesystem path.
REPO_ROOT = os.environ.get(
    "PLATFORM_REPO_ROOT",
    os.path.join(os.path.dirname(__file__), "..", "..", "tests", "fixtures", "sample-repo"),
)
