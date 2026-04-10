For generating a fresh token and injecting it, the Lua filter alone won't cut it — Lua in Envoy can't make outbound HTTP calls to a token endpoint. Here are your real options:
Option 1 — ext_authz (cleanest, no gRPC needed)
Your auth server does the token generation, returns it as a header, Envoy forwards it. HTTP is fine — no gRPC required.
Option 2 — External credential injector sidecar
A small process periodically fetches a token and writes it to a shared file. Envoy's Lua filter reads the file on each request. Zero external call per request, but slightly stale tokens possible.
Option 3 — Token relay service (most control)
Envoy routes through a small proxy you own that fetches + injects the token, then forwards to the real backend.
Option 1 (ext_authz via HTTP) is the standard pattern. Here's the full setup:


1. Envoy config — ext_authz with header forwarding


static_resources:
  listeners:
    - name: listener_0
      address:
        socket_address: { address: 0.0.0.0, port_value: 8080 }
      filter_chains:
        - filters:
            - name: envoy.filters.network.http_connection_manager
              typed_config:
                "@type": type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager
                stat_prefix: ingress_http
                http_filters:
                  - name: envoy.filters.http.ext_authz
                    typed_config:
                      "@type": type.googleapis.com/envoy.extensions.filters.http.ext_authz.v3.ExtAuthz
                      # HTTP (not gRPC) — no gRPC server needed
                      http_service:
                        server_uri:
                          uri: http://token-service:9001
                          cluster: token_service
                          timeout: 3s
                        path_prefix: /auth
                        authorization_request:
                          # Forward these headers from original request to token service
                          allowed_headers:
                            patterns:
                              - exact: "x-request-id"
                              - exact: "x-forwarded-for"
                        authorization_response:
                          # Forward these headers from token service response → upstream
                          allowed_upstream_headers:
                            patterns:
                              - exact: "authorization"   # token service sets this
                              - exact: "x-token-expiry"  # optional metadata
                      failure_mode_allow: false   # block request if token fetch fails
                      include_peer_certificate: false

                  - name: envoy.filters.http.router
                    typed_config:
                      "@type": type.googleapis.com/envoy.extensions.filters.http.router.v3.Router

                route_config:
                  name: local_route
                  virtual_hosts:
                    - name: backend
                      domains: ["*"]
                      routes:
                        - match: { prefix: "/" }
                          route: { cluster: backend_service }

  clusters:
    - name: token_service
      connect_timeout: 3s
      type: STRICT_DNS
      lb_policy: ROUND_ROBIN
      load_assignment:
        cluster_name: token_service
        endpoints:
          - lb_endpoints:
              - endpoint:
                  address:
                    socket_address: { address: token-service, port_value: 9001 }

    - name: backend_service
      connect_timeout: 5s
      type: STRICT_DNS
      lb_policy: ROUND_ROBIN
      load_assignment:
        cluster_name: backend_service
        endpoints:
          - lb_endpoints:
              - endpoint:
                  address:
                    socket_address: { address: backend-host, port_value: 8000 }




import time
import httpx
from fastapi import FastAPI, Request, Response

app = FastAPI()

# ── In-memory token cache ──────────────────────────────────────
_cache = {"token": None, "expires_at": 0}

OAUTH_TOKEN_URL  = "https://your-idp.com/oauth2/token"
CLIENT_ID        = "your-client-id"
CLIENT_SECRET    = "your-client-secret"
SCOPE            = "api.read api.write"
BUFFER_SECONDS   = 60   # refresh this many seconds before actual expiry


async def get_token() -> str:
    """Return cached token, or fetch a new one if expired."""
    now = time.time()
    if _cache["token"] and now < _cache["expires_at"]:
        return _cache["token"]

    async with httpx.AsyncClient() as client:
        resp = await client.post(
            OAUTH_TOKEN_URL,
            data={
                "grant_type":    "client_credentials",
                "client_id":     CLIENT_ID,
                "client_secret": CLIENT_SECRET,
                "scope":         SCOPE,
            },
            headers={"Content-Type": "application/x-www-form-urlencoded"},
            timeout=5.0,
        )
        resp.raise_for_status()
        payload = resp.json()

    _cache["token"]      = payload["access_token"]
    _cache["expires_at"] = now + payload.get("expires_in", 3600) - BUFFER_SECONDS
    return _cache["token"]


# ── ext_authz endpoint ─────────────────────────────────────────
@app.get("/auth")
@app.post("/auth")
async def authorize(request: Request):
    """
    Envoy calls this for every inbound request.
    Return 200 + the Authorization header → Envoy forwards it upstream.
    Return 403 → Envoy blocks the request.
    """
    try:
        token = await get_token()
        return Response(
            status_code=200,
            headers={
                "Authorization": f"Bearer {token}",
                "x-token-expiry": str(int(_cache["expires_at"])),
            },
        )
    except Exception as e:
        # Log and fail closed
        print(f"Token fetch failed: {e}")
        return Response(status_code=403, content=b"token unavailable")


# ── Health check ───────────────────────────────────────────────
@app.get("/healthz")
async def health():
    return {"status": "ok"}


3. Docker Compose to wire it all together

version: "3.9"
services:
  envoy:
    image: envoyproxy/envoy:v1.29-latest
    ports: ["8080:8080"]
    volumes:
      - ./envoy.yaml:/etc/envoy/envoy.yaml
    depends_on: [token-service]

  token-service:
    build: ./token-service   # directory with your FastAPI app + Dockerfile
    environment:
      - CLIENT_ID=your-client-id
      - CLIENT_SECRET=your-client-secret
      - OAUTH_TOKEN_URL=https://your-idp.com/oauth2/token
    ports: ["9001:9001"]

  backend:
    image: your-backend-image
    ports: ["8000:8000"]


Key design decisions in this setup
The token is cached inside the token service process with a 60-second pre-expiry buffer, so Envoy's latency overhead is typically just a local memory lookup — not an IdP call. The failure_mode_allow: false flag in Envoy means if the token service is down, requests are blocked (fail-closed) rather than forwarded without a token. If you want to use gRPC instead of HTTP for the ext_authz protocol, you'd swap http_service for grpc_service in the Envoy config and implement the envoy.service.auth.v3.Authorization proto — but the HTTP approach above is simpler and handles the vast majority of use cases.