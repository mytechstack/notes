"""
Service catalog adapter — team capacity and service topology.

Production note: this is a stub returning fixed sample data. Swap for a
real call to your sprint-tracker API (capacity) and service-catalog API
(topology). Kept intentionally tiny — this adapter has the least logic
of the three because in a real system most of the "work" is the API
client, not the shaping of what it returns.
"""


def get_team_capacity(team: str) -> dict:
    # Stub — replace with a real sprint-tracker API call.
    return {"team": team, "available_points": 3, "sprint": "2026-W32"}


def get_dependency_graph(service: str) -> dict:
    # Stub — replace with a real service-catalog API call.
    catalog = {
        "checkout-service": {
            "depends_on": ["payment-gateway", "inventory-service"],
            "depended_on_by": ["web-frontend", "mobile-api"],
        },
    }
    return catalog.get(service, {"depends_on": [], "depended_on_by": []})
