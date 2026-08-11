"""
MCP server exposing the platform-context-sdk assembly functions as
tools any MCP-capable client can call — GitHub Copilot Chat (Agent
mode) in VS Code/Visual Studio, Claude Code, Cursor, Windsurf, or any
other MCP client.

This is the same assemble_planning_context() / assemble_dev_context()
functions used elsewhere in this SDK — the MCP layer only adds a
transport (stdio) and a tool-schema wrapper. No context-engineering
logic lives in this file; it all stays in assemble.py, owned by the
platform team, reused across every client that speaks MCP.

Run directly for local stdio testing:
    python -m platform_context_sdk.mcp_server

Registered as a server in .vscode/mcp.json (see README) for use from
GitHub Copilot Chat in Agent mode.
"""
from mcp.server.fastmcp import FastMCP

from .assemble import assemble_dev_context, assemble_planning_context

mcp = FastMCP("platform-context")


@mcp.tool()
def get_planning_context(ticket_text: str, team: str) -> str:
    """
    Retrieve scoped planning context for a ticket: related prior
    architecture decisions and current team capacity. Call this before
    scoping or estimating a feature -- do not guess at prior decisions
    from general knowledge.
    """
    return assemble_planning_context(ticket_text=ticket_text, team=team)


@mcp.tool()
def get_dev_context(target_service: str, feature_key: str) -> str:
    """
    Retrieve scoped development context for implementing a config
    feature on a specific service: the platform's schema for that
    feature, one real precedent from another service, and the current
    state of the target file. Call this before writing config changes
    -- do not guess at schema format.
    """
    return assemble_dev_context(target_service=target_service, feature_key=feature_key)


if __name__ == "__main__":
    mcp.run(transport="stdio")
