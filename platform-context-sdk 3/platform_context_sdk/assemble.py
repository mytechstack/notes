"""
Assembly layer — owned by the platform team.

These functions are the actual context-engineering work: SELECTION (which
sources, how much of each), STRUCTURE (labeled, consistently formatted
blocks), COMPRESSION (extracted blocks, not whole files), and ISOLATION
(each source in its own section, tagged with provenance).

Application teams import these functions; they don't reimplement
retrieval or formatting themselves. Keep the return type plain text (or
swap to a structured dict — see `assemble_dev_context_structured` below)
so callers can drop it straight into a model call's system prompt or
first user message.
"""
from .adapters import adr_index, gateway_schema, service_catalog


def assemble_planning_context(ticket_text: str, team: str) -> str:
    """
    Context for the planning phase: the ticket, relevant prior decisions,
    and current team capacity — nothing else.
    """
    adrs = adr_index.get_relevant_adrs(ticket_text, top_k=2)
    capacity = service_catalog.get_team_capacity(team)

    adr_lines = (
        "\n".join(f"- {a.id}: {a.title} — {a.excerpt}" for a in adrs)
        if adrs
        else "(no related prior decisions found)"
    )

    return (
        f"TICKET:\n{ticket_text}\n\n"
        f"RELATED PRIOR DECISIONS (source: ADR index):\n{adr_lines}\n\n"
        f"TEAM CAPACITY (source: sprint tracker):\n"
        f"- {capacity['available_points']} story points available, sprint {capacity['sprint']}\n"
    )


def assemble_dev_context(target_service: str, feature_key: str) -> str:
    """
    Context for the development phase: the platform's config schema for
    this feature, one concrete precedent from another service, and the
    target file being edited — each in its own labeled, isolated block.
    """
    schema_block = gateway_schema.get_schema_block(feature_key)

    precedent_service = gateway_schema.find_precedent(feature_key, exclude_service=target_service)
    precedent_block = (
        gateway_schema.get_service_config_block(precedent_service, feature_key)
        if precedent_service
        else None
    )

    target_raw = gateway_schema.get_target_file_raw(target_service)

    sections = [
        f"CONVENTIONS (source: platform/schemas/gateway-config.schema.yaml):\n{schema_block}",
    ]

    if precedent_block:
        sections.append(
            f"EXISTING PRECEDENT (source: services/{precedent_service}/gateway.yaml):\n{precedent_block}"
        )
    else:
        sections.append("EXISTING PRECEDENT: (no other service has this config yet)")

    sections.append(
        f"TARGET FILE (source: services/{target_service}/gateway.yaml):\n{target_raw}"
    )

    return "\n\n".join(sections) + "\n"


def assemble_dev_context_structured(target_service: str, feature_key: str) -> dict:
    """
    Same assembly as `assemble_dev_context`, returned as a structured dict
    instead of a flat string. Useful when the caller wants to log
    provenance separately, run evals on individual fields, or format
    the sections differently (e.g. as XML tags) for a specific model.
    """
    schema_block = gateway_schema.get_schema_block(feature_key)
    precedent_service = gateway_schema.find_precedent(feature_key, exclude_service=target_service)
    precedent_block = (
        gateway_schema.get_service_config_block(precedent_service, feature_key)
        if precedent_service
        else None
    )
    target_raw = gateway_schema.get_target_file_raw(target_service)

    return {
        "conventions": {
            "source": "platform/schemas/gateway-config.schema.yaml",
            "content": schema_block,
        },
        "precedent": {
            "source": f"services/{precedent_service}/gateway.yaml" if precedent_service else None,
            "content": precedent_block,
        },
        "target_file": {
            "source": f"services/{target_service}/gateway.yaml",
            "content": target_raw,
        },
    }
