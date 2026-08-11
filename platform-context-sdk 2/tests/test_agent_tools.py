"""
Tests for the agent-facing tool layer. These check the CONTRACT an
agent runtime depends on: valid schemas, correct dispatch, and — this
is the one that matters most for agents — that tool results stay
small and scoped rather than silently growing.
"""
import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from platform_context_sdk.agent_tools import TOOL_SCHEMAS, call_tool


class TestToolSchemas:
    def test_all_tools_have_required_fields(self):
        for tool in TOOL_SCHEMAS:
            assert "name" in tool
            assert "description" in tool
            assert "input_schema" in tool
            assert tool["input_schema"]["type"] == "object"

    def test_tool_names_are_unique(self):
        names = [t["name"] for t in TOOL_SCHEMAS]
        assert len(names) == len(set(names))


class TestToolDispatch:
    def test_planning_tool_dispatches_correctly(self):
        result = call_tool(
            "get_planning_context",
            {"ticket_text": "Add rate limiting to checkout-service API", "team": "checkout-team"},
        )
        assert "TICKET:" in result
        assert "ADR-142" in result

    def test_dev_tool_dispatches_correctly(self):
        result = call_tool(
            "get_dev_context",
            {"target_service": "checkout-service", "feature_key": "rate_limit"},
        )
        assert "CONVENTIONS" in result
        assert "auth-service" in result

    def test_unknown_tool_raises(self):
        try:
            call_tool("nonexistent_tool", {})
            assert False, "expected ValueError"
        except ValueError:
            pass

    def test_tool_result_stays_scoped_not_whole_repo(self):
        # An agent calling get_dev_context should NOT get back content
        # from unrelated services (e.g. inventory-service) — the result
        # should be scoped to just what this step needs.
        result = call_tool(
            "get_dev_context",
            {"target_service": "checkout-service", "feature_key": "rate_limit"},
        )
        assert "inventory-service" not in result

    def test_sequential_calls_dont_leak_between_steps(self):
        # Step 1's result and step 2's result should each be independently
        # scoped — step 2 shouldn't accidentally include step 1's content
        # just because they ran in the same process.
        planning = call_tool(
            "get_planning_context",
            {"ticket_text": "Add rate limiting to checkout-service API", "team": "checkout-team"},
        )
        dev = call_tool(
            "get_dev_context",
            {"target_service": "checkout-service", "feature_key": "rate_limit"},
        )
        assert "TEAM CAPACITY" not in dev
        assert "CONVENTIONS" not in planning
