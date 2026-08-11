# platform-context-sdk

Shared context-engineering infrastructure, owned by the platform team.
Application teams import these functions instead of hand-rolling their
own retrieval, formatting, and compression logic per project.

## Ownership model

| | Owns |
|---|---|
| **Platform team** | `adapters/` (source-system integrations), `assemble.py` (selection, structure, compression, isolation logic), the eval suite in `tests/` |
| **Application teams** | Calling `assemble_*()` with their inputs, and the prompt/instructions layered on top for their specific bot |

Application teams should never need to know how ADRs are indexed or how
the gateway schema is stored — they call `assemble_dev_context(service, feature)`
and get back model-ready text.

## Structure

```
platform_context_sdk/
├── adapters/              # one module per source system
│   ├── adr_index.py         — architecture decision records
│   ├── gateway_schema.py    — config schema + per-service configs
│   └── service_catalog.py   — team capacity, dependency graphs
├── assemble.py             # the actual context-engineering functions
tests/
├── fixtures/sample-repo/   # a mock repo the adapters read from
└── test_assemble.py        # evals — checks assembly QUALITY, not just that code runs
examples/
└── run_example.py          # what an application team's call site looks like
```

## Quickstart

```bash
pip install pyyaml pytest --break-system-packages   # or use a venv
python examples/run_example.py                       # see assembled context
pytest tests/ -v                                      # run the eval suite
```

## Using it in your own bot

```python
from platform_context_sdk import assemble_dev_context

context = assemble_dev_context(target_service="my-service", feature_key="rate_limit")

# then hand it to a model call
response = model.call(
    system="You are a platform engineering assistant. Use only the context below.\n\n" + context,
    messages=[{"role": "user", "content": "Add rate limiting to my service"}],
)
```

## Swapping in real infrastructure

Every adapter in `adapters/` currently reads from `tests/fixtures/sample-repo/`
so this SDK runs with zero external dependencies. To point it at real
systems, replace the body of each adapter function — keep the signature
the same so callers don't need to change:

| Adapter | Replace with |
|---|---|
| `adr_index.get_relevant_adrs()` | Your doc-search / embeddings index |
| `gateway_schema.get_schema_block()` | Your config-management API |
| `service_catalog.get_team_capacity()` | Your sprint-tracker API |
| `service_catalog.get_dependency_graph()` | Your service-catalog API |

Set `PLATFORM_REPO_ROOT` env var to point adapters at a different fixture
root, or replace the file-based adapters entirely with API clients once
real integrations exist.

## Using with GitHub Copilot (VS Code / Visual Studio, Agent mode)

Copilot Chat supports MCP servers natively in Agent mode (GA since
VS Code 1.102, mid-2025). This SDK ships an MCP server
(`platform_context_sdk/mcp_server.py`) that exposes the same
`assemble_planning_context` / `assemble_dev_context` functions as
tools Copilot can call mid-task — the same tools shown in the coding
agent demo above, just reachable from Copilot instead of a custom loop.

1. Install dependencies:
   ```bash
   pip install -r requirements.txt --break-system-packages
   ```

2. This repo already includes `.vscode/mcp.json`:
   ```json
   {
     "servers": {
       "platform-context": {
         "type": "stdio",
         "command": "python",
         "args": ["-m", "platform_context_sdk.mcp_server"],
         "cwd": "${workspaceFolder}"
       }
     }
   }
   ```
   If wiring this into an existing repo instead, copy this block into
   that repo's own `.vscode/mcp.json` (create the file if it doesn't
   exist) and commit it so the whole team gets it.

3. Open this folder in VS Code. Open Copilot Chat, switch the mode
   dropdown from **Ask** to **Agent**. VS Code will detect the server
   and show a **Start** button (or run `MCP: List Servers` from the
   Command Palette to start it manually).

4. Ask Copilot something that needs this context, e.g.:
   > "Add rate limiting to checkout-service, following our conventions"

   Copilot will show a tool-confirmation prompt for `get_dev_context`
   (or `get_planning_context`) before calling it — click **Continue**.
   The assembled context comes back as the tool result and Copilot
   reasons over it the same way the simulated agent loop does in
   `examples/agent_loop_demo.py`.

5. Verify the server works standalone before trusting it in Copilot:
   ```bash
   python tests/test_mcp_server_live.py
   ```
   This connects a real MCP client over stdio, lists the registered
   tools, and calls both — useful for debugging without going through
   the VS Code UI.

**Enterprise note:** if your org is on Copilot Business/Enterprise, an
admin may need to enable the "MCP servers in Copilot" policy before
this works — see GitHub's MCP docs if the server doesn't appear.



1. Add or extend an adapter in `adapters/` for any new source system
2. Write the `assemble_*()` function in `assemble.py` — select, structure,
   compress, isolate
3. Add eval-style tests in `tests/test_assemble.py`: does it retrieve the
   right thing, exclude the wrong thing, keep sections isolated
4. Application teams pick it up on the next SDK version — no changes
   needed on their end beyond importing the new function
