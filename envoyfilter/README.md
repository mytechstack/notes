# Envoy OAuth Token Injection Demo

Demonstrates using Envoy's Lua HTTP filter to fetch a fresh OAuth token on every request and inject it as the `Authorization` header before forwarding to a backend service.

## Architecture

```
Client
  │
  ▼  :8080
Envoy Proxy
  │  (Lua filter: POST /token → get JWT → replace Authorization header)
  │
  ├──► oauth-server   :8082   issues HS256 JWTs (client_credentials grant)
  │
  └──► backend-service :8081  validates JWT, returns request dump
```

**Flow for every request:**
1. Client sends a request to Envoy (no auth needed from the client)
2. Envoy's Lua filter calls the OAuth server with `client_credentials`
3. Envoy replaces (or adds) the `Authorization: Bearer <jwt>` header
4. Envoy forwards the enriched request to the backend
5. Backend validates the JWT and responds

## Prerequisites

- [Docker Desktop](https://www.docker.com/products/docker-desktop/) running

## Quick start

```bash
# Clone / enter the directory
cd envoyfilter

# Build and start all three containers
docker compose up --build

# In a second terminal, run the automated tests
bash client/test.sh
```

Services come up in dependency order: OAuth server and backend first, then Envoy.

## Manual curl examples

```bash
# 1. Hit Envoy — no Authorization header needed from the client
curl http://localhost:8080/api/hello

# 2. Send a fake token — Envoy replaces it with a real one
curl http://localhost:8080/api/secure \
  -H "Authorization: Bearer FAKE-TOKEN"

# 3. POST with a JSON body
curl -X POST http://localhost:8080/api/data \
  -H "Content-Type: application/json" \
  -d '{"key": "value"}'

# 4. Fetch a token directly from the OAuth server
curl -X POST http://localhost:8082/token \
  -d "grant_type=client_credentials&client_id=envoy-proxy&client_secret=envoy-secret"

# 5. Verify a token directly
curl http://localhost:8082/verify \
  -H "Authorization: Bearer <token>"
```

## Ports

| Port | Service |
|------|---------|
| 8080 | Envoy proxy (send your requests here) |
| 8081 | Backend service (direct access) |
| 8082 | OAuth server (direct access) |
| 9901 | Envoy admin dashboard |

## Envoy admin dashboard

Open `http://localhost:9901` in a browser to inspect Envoy's live state.

Useful endpoints:

```bash
# Cluster health and stats
curl http://localhost:9901/clusters

# Active listeners
curl http://localhost:9901/listeners

# Live config dump
curl http://localhost:9901/config_dump
```

## How the token injection works

`envoy/envoy.yaml` attaches a Lua filter to the HTTP connection manager:

```lua
function envoy_on_request(request_handle)
  local _, headers, body = request_handle:httpCall(
    "oauth_server",          -- Envoy cluster name
    { [":method"] = "POST", [":path"] = "/token", ... },
    "grant_type=client_credentials&client_id=envoy-proxy&client_secret=envoy-secret",
    5000                     -- timeout ms
  )
  local token = body:match('"access_token"%s*:%s*"([^"]+)"')
  request_handle:headers():replace("authorization", "Bearer " .. token)
  request_handle:headers():add("x-envoy-injected-token", "true")
end
```

`httpCall` routes through Envoy's own connection pool to the `oauth_server` cluster, so it respects Envoy's timeouts, retries, and circuit-breaking config.

## Stopping

```bash
docker compose down
```
