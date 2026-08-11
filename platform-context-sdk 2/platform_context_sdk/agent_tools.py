"""
Exposes the assembly layer as tools a coding agent can call directly,
rather than context a human pre-loads into a static prompt.

This is the key shift for agents: instead of "here is everything you
might need" stuffed into one system prompt, the agent decides WHEN to
call assemble_planning_context or assemble_dev_context as it works
through its own plan -> act -> observe loop. Each call returns freshly
scoped context for the step the agent is currently on — not an ever-
growing transcript of every file it has looked at so far.

TOOL_SCHEMAS below is in Anthropic tool-use format and can be passed
directly to a model.call(tools=TOOL_SCHEMAS) — this is exactly what
Claude Code or any MCP-based agent would register.
"""
from .assemble import assemble_dev_context, assemble_planning_context

TOOL_SCHEMAS = [
    {
        "name": "get_planning_context",
        "description": (
            "Retrieve scoped planning context for a ticket: related prior "
            "architecture decisions and current team capacity. Call this "
            "before scoping or estimating a feature — do not guess at "
            "prior decisions from general knowledge."
        ),
        "input_schema": {
            "type": "object",
            "properties": {
                "ticket_text": {"type": "string", "description": "The ticket or feature request text"},
                "team": {"type": "string", "description": "Owning team, e.g. 'checkout-team'"},
            },
            "required": ["ticket_text", "team"],
        },
    },
    {
        "name": "get_dev_context",
        "description": (
            "Retrieve scoped development context for implementing a config "
            "feature on a specific service: the platform's schema for that "
            "feature, one real precedent from another service, and the "
            "current state of the target file. Call this before writing "
            "config changes — do not guess at schema format."
        ),
        "input_schema": {
            "type": "object",
            "properties": {
                "target_service": {"type": "string", "description": "Service being modified, e.g. 'checkout-service'"},
                "feature_key": {"type": "string", "description": "Config key being added, e.g. 'rate_limit'"},
            },
            "required": ["target_service", "feature_key"],
        },
    },
]


def call_tool(name: str, tool_input: dict) -> str:
    """
    Dispatcher an agent runtime calls when the model requests one of the
    tools above. Returns the assembled context as plain text — this is
    the tool_result that goes back into the agent's next model call.
    """
    if name == "get_planning_context":
        return assemble_planning_context(
            ticket_text=tool_input["ticket_text"],
            team=tool_input["team"],
        )
    if name == "get_dev_context":
        return assemble_dev_context(
            target_service=tool_input["target_service"],
            feature_key=tool_input["feature_key"],
        )
    raise ValueError(f"Unknown tool: {name}")
