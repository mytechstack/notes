"""
Tests for the assembly layer.

These aren't just "does the code run" tests — several check assembly
QUALITY: did selection pull the right ADR and exclude the wrong one, did
isolation keep sources in separate blocks, did compression avoid leaking
a whole file when only one block was needed. This is the "eval context,
not just output" idea applied concretely.
"""
import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from platform_context_sdk.assemble import (
    assemble_dev_context,
    assemble_dev_context_structured,
    assemble_planning_context,
)


class TestPlanningContext:
    def test_selects_relevant_adr(self):
        ctx = assemble_planning_context(
            ticket_text="Add rate limiting to checkout-service API",
            team="checkout-team",
        )
        assert "ADR-142" in ctx
        assert "rate limiting" in ctx.lower()

    def test_excludes_irrelevant_adr(self):
        # ADR-098 is about logging, not rate limiting — selection should
        # filter it out rather than including everything in the index.
        ctx = assemble_planning_context(
            ticket_text="Add rate limiting to checkout-service API",
            team="checkout-team",
        )
        assert "ADR-098" not in ctx
        assert "logging" not in ctx.lower()

    def test_includes_capacity(self):
        ctx = assemble_planning_context(ticket_text="Add rate limiting", team="checkout-team")
        assert "story points" in ctx

    def test_sections_are_labeled(self):
        ctx = assemble_planning_context(ticket_text="Add rate limiting", team="checkout-team")
        assert "TICKET:" in ctx
        assert "RELATED PRIOR DECISIONS" in ctx
        assert "TEAM CAPACITY" in ctx


class TestDevContext:
    def test_finds_correct_precedent(self):
        # auth-service has rate_limit configured; inventory-service does not.
        # Selection should find auth-service, not inventory-service.
        ctx = assemble_dev_context(target_service="checkout-service", feature_key="rate_limit")
        assert "auth-service" in ctx
        assert "services/auth-service/gateway.yaml" in ctx

    def test_precedent_excludes_target_service(self):
        # find_precedent must never return the service being edited.
        ctx = assemble_dev_context(target_service="checkout-service", feature_key="rate_limit")
        assert "services/checkout-service/gateway.yaml" in ctx  # appears as TARGET FILE
        # but checkout-service should not appear as the precedent's source
        precedent_section = ctx.split("EXISTING PRECEDENT")[1].split("TARGET FILE")[0]
        assert "checkout-service" not in precedent_section

    def test_isolation_keeps_sections_separate(self):
        ctx = assemble_dev_context(target_service="checkout-service", feature_key="rate_limit")
        assert "CONVENTIONS" in ctx
        assert "EXISTING PRECEDENT" in ctx
        assert "TARGET FILE" in ctx
        # sections appear in a sane order: conventions, then precedent, then target
        assert ctx.index("CONVENTIONS") < ctx.index("EXISTING PRECEDENT") < ctx.index("TARGET FILE")

    def test_compression_excludes_unrelated_config_blocks(self):
        # auth-service's gateway.yaml also has retries/timeouts/auth blocks.
        # Only rate_limit should be pulled — not the whole file.
        ctx = assemble_dev_context(target_service="checkout-service", feature_key="rate_limit")
        precedent_section = ctx.split("EXISTING PRECEDENT")[1].split("TARGET FILE")[0]
        assert "rate_limit" in precedent_section
        assert "retries" not in precedent_section
        assert "timeouts" not in precedent_section

    def test_target_file_shows_current_state(self):
        ctx = assemble_dev_context(target_service="checkout-service", feature_key="rate_limit")
        # checkout-service's current gateway.yaml has no rate_limit block —
        # the model should see that absence, not a guess.
        target_section = ctx.split("TARGET FILE")[1]
        assert "rate_limit" not in target_section

    def test_structured_variant_matches_flat_variant_content(self):
        flat = assemble_dev_context(target_service="checkout-service", feature_key="rate_limit")
        structured = assemble_dev_context_structured(target_service="checkout-service", feature_key="rate_limit")
        assert structured["precedent"]["source"] == "services/auth-service/gateway.yaml"
        assert "auth-service" in flat

    def test_missing_precedent_handled_explicitly(self):
        # No service has an "auth" block precedent besides the target
        # itself in this fixture set for a feature nobody else has yet.
        ctx = assemble_dev_context(target_service="checkout-service", feature_key="circuit_breaker")
        assert "no other service has this config yet" in ctx
