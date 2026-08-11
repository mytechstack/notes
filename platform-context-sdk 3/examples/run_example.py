"""
Example: how an application team uses the platform-owned SDK.

This is what "checkout-service-bot" would actually call — it doesn't
know or care how ADRs are indexed or how the schema store works, it
just calls the assembly function and gets back model-ready context.

Run: python examples/run_example.py
"""
import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from platform_context_sdk import assemble_dev_context, assemble_planning_context

if __name__ == "__main__":
    print("=" * 70)
    print("PLANNING PHASE — assembled context")
    print("=" * 70)
    planning_ctx = assemble_planning_context(
        ticket_text="Add rate limiting to checkout-service API",
        team="checkout-team",
    )
    print(planning_ctx)

    print("=" * 70)
    print("DEVELOPMENT PHASE — assembled context")
    print("=" * 70)
    dev_ctx = assemble_dev_context(target_service="checkout-service", feature_key="rate_limit")
    print(dev_ctx)

    print("=" * 70)
    print("This is what actually gets sent to the model.")
    print("The application team never touched ADR retrieval, schema")
    print("parsing, or precedent-finding logic — that's all in the SDK.")
    print("=" * 70)
