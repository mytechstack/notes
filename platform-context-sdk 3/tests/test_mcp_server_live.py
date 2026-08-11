"""
Connects to mcp_server.py as a real MCP client would (stdio transport),
lists its tools, and calls one — proving the server actually works end
to end, the same way GitHub Copilot Chat's Agent mode would.
"""
import asyncio
import os
import sys

from mcp import ClientSession, StdioServerParameters
from mcp.client.stdio import stdio_client

REPO_ROOT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..")


async def main():
    server_params = StdioServerParameters(
        command=sys.executable,
        args=["-m", "platform_context_sdk.mcp_server"],
        cwd=REPO_ROOT,
    )

    async with stdio_client(server_params) as (read, write):
        async with ClientSession(read, write) as session:
            await session.initialize()

            tools_result = await session.list_tools()
            print("=== Tools registered on the MCP server ===")
            for tool in tools_result.tools:
                print(f"- {tool.name}: {tool.description.strip().splitlines()[0]}")

            print("\n=== Calling get_planning_context via MCP (as Copilot would) ===")
            result = await session.call_tool(
                "get_planning_context",
                {"ticket_text": "Add rate limiting to checkout-service API", "team": "checkout-team"},
            )
            for block in result.content:
                if hasattr(block, "text"):
                    print(block.text)

            print("=== Calling get_dev_context via MCP ===")
            result = await session.call_tool(
                "get_dev_context",
                {"target_service": "checkout-service", "feature_key": "rate_limit"},
            )
            for block in result.content:
                if hasattr(block, "text"):
                    print(block.text)


if __name__ == "__main__":
    asyncio.run(main())
