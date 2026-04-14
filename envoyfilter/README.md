# Envoy OAuth Token Injection Demo

Demonstrates using Envoy's **ext_proc gRPC filter** to inject a fresh, cached OAuth token into every upstream request — without the client needing to authenticate at all.

## Architecture

```
Client
  │
  ▼  :8080
Envoy Proxy
  │  ext_proc filter — opens a gRPC stream per request
  │
  ├──► token-manager  :9090 (gRPC)   Go service — caches JWT, fetches only on expiry
  │                   :8083 (HTTP)   /health endpoint
  │         │
  │         └──► oauth-server  :8082   issues HS256 JWTs (client_credentials grant)
  │
  └──► backend-service :8081   validates JWT, echoes request headers
```

**Flow for every request:**
1. Client sends a request to Envoy — no `Authorization` header required
2. Envoy opens a gRPC stream to the token-manager (`ExternalProcessor/Process`)
3. Token-manager returns a `HeaderMutation` with `Authorization: Bearer <jwt>`
4. Envoy applies the mutation and forwards the enriched request to the backend
5. Backend validates the JWT and responds

Token caching: the token-manager fetches from the OAuth server **once**, serves from memory, and only refreshes within 60 s of expiry.

## Prerequisites

- [Docker Desktop](https://www.docker.com/products/docker-desktop/) running

## Quick start

```bash
cd envoyfilter

# Build all images and start all four containers
docker compose up --build

# In a second terminal, run the automated test suite
bash client/test.sh
```

Services start in dependency order: OAuth server → token-manager → backend → Envoy.

## Manual curl examples

```bash
# 1. Hit Envoy — no Authorization header from the client
curl http://localhost:8080/api/hello

# 2. Send a fake token — Envoy replaces it with a real one
curl http://localhost:8080/api/secure \
  -H "Authorization: Bearer FAKE-TOKEN"

# 3. POST with a body — body passes through, token is still injected
curl -X POST http://localhost:8080/api/data \
  -H "Content-Type: application/json" \
  -d '{"key": "value"}'

# 4. Check the token-manager cache stats (fetch_count should stay at 1)
curl http://localhost:8083/health

# 5. Fetch a token directly from the OAuth server
curl -X POST http://localhost:8082/token \
  -d "grant_type=client_credentials&client_id=envoy-proxy&client_secret=envoy-secret"

# 6. Verify a token directly on the OAuth server
curl http://localhost:8082/verify \
  -H "Authorization: Bearer <token>"
```

## Ports

| Port | Service | Notes |
|------|---------|-------|
| 8080 | Envoy proxy | Send all client requests here |
| 8081 | Backend service | Direct access for debugging |
| 8082 | OAuth server | Direct access for debugging |
| 8083 | Token-manager HTTP | `/health` — cache stats |
| 9090 | Token-manager gRPC | Envoy ext_proc connects here |
| 9901 | Envoy admin dashboard | Stats, config dump |

## Envoy admin dashboard

Open `http://localhost:9901` in a browser to inspect live state.

```bash
# Cluster health and connection stats
curl http://localhost:9901/clusters

# Active listeners
curl http://localhost:9901/listeners

# Full config dump
curl http://localhost:9901/config_dump
```

## How the token injection works

### Envoy config (`envoy/envoy.yaml`)

The `ext_proc` filter intercepts every request and calls the token-manager over gRPC. Only the request-headers phase is sent (`SEND`); body and response are skipped (`SKIP`/`NONE`) to keep overhead minimal.

```yaml
- name: envoy.filters.http.ext_proc
  typed_config:
    "@type": type.googleapis.com/envoy.extensions.filters.http.ext_proc.v3.ExternalProcessor
    grpc_service:
      envoy_grpc:
        cluster_name: token_manager_grpc
    processing_mode:
      request_header_mode: SEND
      response_header_mode: SKIP
    message_timeout: 5s
    failure_mode_allow: false   # block request if token-manager is down
```

The `token_manager_grpc` cluster must use HTTP/2 (required for gRPC):

```yaml
typed_extension_protocol_options:
  envoy.extensions.upstreams.http.v3.HttpProtocolOptions:
    "@type": type.googleapis.com/envoy.extensions.upstreams.http.v3.HttpProtocolOptions
    explicit_http_config:
      http2_protocol_options: {}
```

### Token-manager (`token-manager/main.go`)

A Go gRPC service implementing `envoy.service.ext_proc.v3.ExternalProcessor`. For each stream (one per HTTP request) it:

1. Receives `ProcessingRequest{RequestHeaders}`
2. Returns a cached token (or fetches a fresh one from OAuth)
3. Sends back `ProcessingResponse` with a `HeaderMutation`

```go
stream.Send(&extprocv3.ProcessingResponse{
    Response: &extprocv3.ProcessingResponse_RequestHeaders{
        RequestHeaders: &extprocv3.HeadersResponse{
            Response: &extprocv3.CommonResponse{
                HeaderMutation: &extprocv3.HeaderMutation{
                    SetHeaders: []*corev3.HeaderValueOption{
                        {Header: &corev3.HeaderValue{
                            Key:      "authorization",
                            RawValue: []byte("Bearer " + token), // RawValue required in Envoy v1.29+
                        }},
                    },
                },
            },
        },
    },
})
```

> **Note:** Use `RawValue` (bytes), not `Value` (string). The `HeaderValue.value` string field is deprecated in Envoy v1.29 — setting it produces an empty header on the wire.

> **Note:** Avoid the `x-envoy-*` prefix for custom headers. Envoy sanitizes all `x-envoy-*` headers to prevent downstream spoofing, even when set by ext_proc.

## Stopping

```bash
docker compose down
```
