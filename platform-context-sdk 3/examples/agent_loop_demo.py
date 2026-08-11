"""
Simulates how a coding agent (Claude Code, an MCP-based agent, etc.)
would actually use this SDK — not as a human pre-loading context, but
as tools the agent decides to call at each step of its own loop.

No real model call here (to keep this runnable with zero API keys) —
the "agent" is a scripted plan showing WHICH tool it calls at each step
and what comes back. In a real agent, the decision of which tool to
call and when comes from the model itself, driven by the task.

Run: python examples/agent_loop_demo.py
"""
import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from platform_context_sdk.agent_tools import TOOL_SCHEMAS, call_tool

TASK = "Add rate limiting to checkout-service API"


def agent_step(step_num: int, thought: str, tool_name: str, tool_input: dict):
    print(f"\n--- Agent step {step_num} ---")
    print(f"[agent reasoning] {thought}")
    print(f"[tool call] {tool_name}({tool_input})")
    result = call_tool(tool_name, tool_input)
    print(f"[tool result — {len(result)} chars returned to agent context]")
    print(result)
    return result


if __name__ == "__main__":
    print("=" * 70)
    print(f"TASK: {TASK}")
    print("Registered tools:", [t["name"] for t in TOOL_SCHEMAS])
    print("=" * 70)

    # Step 1 — agent decides it needs planning context before scoping work.
    # It does NOT get this dumped into its system prompt up front; it
    # asks for it because the task requires it.
    planning_ctx = agent_step(
        1,
        "Before scoping this ticket, check for related prior decisions and capacity.",
        "get_planning_context",
        {"ticket_text": TASK, "team": "checkout-team"},
    )

    # Step 2 — agent has decided (from planning_ctx) to reuse the
    # token-bucket pattern from ADR-142, and now needs the actual schema
    # + a real precedent before writing config. It calls a DIFFERENT
    # tool for a DIFFERENT step — it doesn't re-request planning context.
    dev_ctx = agent_step(
        2,
        "Ready to implement. Need the config schema and a working precedent.",
        "get_dev_context",
        {"target_service": "checkout-service", "feature_key": "rate_limit"},
    )

    # Step 3 — agent generates the actual change using ONLY what step 2
    # returned, not the accumulated output of steps 1+2 concatenated.
    print("\n--- Agent step 3 ---")
    print("[agent reasoning] Generating config change from dev context only.")
    print("[generated diff]")
    print("""--- a/services/checkout-service/gateway.yaml
+++ b/services/checkout-service/gateway.yaml
@@ -1,4 +1,8 @@
 service: checkout-service

+rate_limit:
+  strategy: token_bucket
+  scope: per-endpoint
+  threshold: 50/min
+
 retries:
   max_attempts: 2""")

    print("\n" + "=" * 70)
    print("Note what did NOT happen: step 3's context window does not")
    print("contain the raw ADR file, the raw schema file, or the raw")
    print("auth-service file — only the pre-shaped output of step 2's")
    print("tool call. Each step's context is scoped to that step.")
    print("=" * 70)
