# Context Hydrator — Platform Architecture

## Overview

Context Hydrator is a **platform-level cache pre-hydration service**. It eagerly fetches user context (profile, preferences, permissions, resources) from upstream backend services and stores it in Redis **before the user authenticates**. Downstream apps read from Redis at sub-millisecond latency instead of hitting backends on every request.

The platform is designed to be consumed by any app team within the organisation. App teams onboard self-service — the platform team owns the infrastructure, SDKs, and APIs. App teams own their backend URLs, resource definitions, and TTL preferences.

---

## The Four Layers

```
┌─────────────────────────────────────────────────────────┐
│  Layer 4: Developer Experience                           │
│  Go/TypeScript SDK, self-service registration API, docs  │
├─────────────────────────────────────────────────────────┤
│  Layer 3: Platform API                                   │
│  hydration-service, context-reader-service, app-registry │
├─────────────────────────────────────────────────────────┤
│  Layer 2: Multi-tenant Infrastructure                    │
│  per-app config, secrets, Redis namespacing, rate limits │
├─────────────────────────────────────────────────────────┤
│  Layer 1: Core Engine                                    │
│  token validation, parallel hydration, cache write/read  │
└─────────────────────────────────────────────────────────┘
```

---

## Services

### hydration-service (unauthenticated)

Handles pre-auth cache warming. No session required — validated by the persistent hydration JWT only.

```
POST /hydrate
     Header: X-App-ID: <appID>
     Body:   { "token": "<signed JWT>" }
```

Responsibilities:
- Verify JWT signature and expiry using per-app secret
- Resolve opaque `hyd_token` → `{ contextKey, claims }` via Redis mapping
- Load app config (backend URL templates, resource list)
- Fan out parallel fetches to app's backend services
- Write results to Redis under namespaced keys
- Return `202 Accepted` with no body

### context-reader-service (authenticated)

Serves cached data to authenticated callers. Redis is its only dependency — no backend access.

```
GET  /data/{contextKey}/{resource}
GET  /context/{contextKey}?resources=profile,preferences,...
     Header: X-App-ID: <appID>
     Header: Authorization: Bearer <session-token>
```

Responsibilities:
- Verify session token
- Read from Redis using `appID:resource:contextKey` key
- Return `404` on cache miss — caller is responsible for triggering re-hydration
- Return `X-Cache: HIT` header

### app-registry-service

Self-service onboarding for app teams.

```
POST   /platform/apps/register
GET    /platform/apps/{appID}
PUT    /platform/apps/{appID}
DELETE /platform/apps/{appID}
```

---

## Security Model

### Two-token architecture

The security guarantee rests on two tokens with **non-overlapping powers**. Neither token alone is sufficient to read user data.

```
Persistent hydration JWT  →  POST /hydrate only         (pre-auth)
Session token             →  GET /data, GET /context    (post-auth)
```

An attacker with only the hydration JWT can trigger cache warming — but the cached data is unreachable without a valid session token.

### Persistent hydration JWT

Issued at login. Survives logout intentionally — it exists to warm the cache on returning visits before re-authentication.

```
Header:  { "alg": "HS256", "typ": "JWT" }
Payload: { "hyd_token": "<opaque>", "app_id": "<appID>", "iat": ..., "exp": ... }
```

Properties:
- `hyd_token` is opaque — contains no user identity, no PII
- Signed with per-app HMAC secret stored in AWS Secrets Manager
- `exp` set to 30 days; re-issued on each login (overwrites browser cookie)
- Cookie flags: `HttpOnly; Secure; SameSite=Strict; Path=/hydrate`
- `Path=/hydrate` — browser sends this cookie only to `/hydrate`, nowhere else

### Opaque token generation

The `hyd_token` is derived deterministically from the app's `contextKey` using HMAC:

```
hyd_token = HMAC-SHA256(contextKey, appSecret)
```

- Same `contextKey` + same secret always produces the same token
- Backend resolves token → claims via Redis mapping
- No user identity is derivable from the token itself

### JWT validation sequence on every `/hydrate` call

```
1. Verify HS256 signature        — reject if tampered or forged
2. Reject non-HMAC algorithms    — blocks alg:none attack
3. Check exp claim               — reject if expired
4. Extract hyd_token + app_id    — reject if either missing
5. Load app config for app_id    — reject if unknown app
6. Redis: resolve hyd_token      — reject if not found or expired
7. Proceed with hydration
```

### Attack surface analysis

`/hydrate` is the only internet-facing unauthenticated endpoint. All other endpoints require a session token and sit behind the auth layer.

| Attack | Result |
|---|---|
| Forge hydration JWT | Fails — requires per-app HMAC secret |
| Tamper JWT payload | Fails — signature check rejects it |
| Steal JWT, trigger hydration | Cache warms — attacker cannot read data without session token |
| Enumerate user identities | Fails — `hyd_token` is opaque, backend rejects unknown tokens |
| Read cached data pre-auth | Fails — `/data` requires session token, not internet-facing |
| Replay after logout | By design — only warms cache, harmless without session token |
| Cross-app data access | Fails — Redis keys namespaced by `appID` |
| DDoS on `/hydrate` | Mitigated — WAF + IP rate limit at gateway, token rate limit at service |
| Backend hammering via flood | Mitigated — invalid JWTs rejected before backend is called; token-level rate limit caps valid requests |
| Large JWT payload attack | Mitigated — WAF rejects oversized payloads before they reach the service |

---

## Multi-tenancy

### contextKey — app-defined identity

Different apps have different identity constructs. The platform does not impose a fixed set of identifiers. Each app defines what makes a unique hydration context:

```
App with profiles:      contextKey = userID + ":" + profileID
App without profiles:   contextKey = userID
Multi-tenant app:       contextKey = userID + ":" + tenantID
Org-aware app:          contextKey = userID + ":" + orgID
```

The platform treats `contextKey` as an opaque string. The app defines how it is composed from its claims.

### Profile switching

When a user switches profiles, a new auth token is issued with a new `profileID`. The hydration token is re-derived:

```
HMAC("u1:pA", secret) → tokenA    — profile A session
HMAC("u1:pB", secret) → tokenB    — profile B session
```

Each profile has its own:
- Hydration token
- Redis mapping entry
- Namespaced cache keys
- Independent TTL

Switching back to a previous profile finds the cache still warm (within TTL), giving near-instant hydration.

### Redis namespacing

All cache keys are prefixed with `appID` to prevent cross-app data collisions:

```
payments-app:profile:u1:acc-99
payments-app:limits:u1:acc-99
identity-app:profile:u1
identity-app:preferences:u1
```

### Redis mapping

At login, the app stores the `hyd_token → claims` mapping:

```
Key:   hyd:mapping:<appID>:<hyd_token>
Value: { "context_key": "u1:pA", "claims": { "user_id": "u1", "profile_id": "pA" } }
TTL:   30 days
```

Hydration service resolves this mapping to obtain the claims needed to build backend URLs.

---

## App Registration

App teams register once. No platform team involvement required after initial setup.

### Registration request

```json
POST /platform/apps/register
{
  "app_id": "payments-app",
  "display_name": "Payments Platform",
  "context_key_claims": ["user_id", "account_id"],
  "resources": {
    "profile": {
      "url": "https://payments.internal/users/{user_id}/profile",
      "ttl": "12h"
    },
    "limits": {
      "url": "https://payments.internal/accounts/{account_id}/limits",
      "ttl": "5m"
    },
    "preferences": {
      "url": "https://payments.internal/users/{user_id}/preferences",
      "ttl": "4h"
    }
  },
  "hydration_token_ttl": "30d",
  "rate_limit": 100
}
```

### Registration response

```json
{
  "app_id": "payments-app",
  "client_secret_arn": "arn:aws:secretsmanager:.../payments-app/hydration-secret",
  "hydration_endpoint": "https://hydrator.platform.internal/hydrate",
  "reader_endpoint":    "https://reader.platform.internal/data"
}
```

### App config schema (stored in registry)

| Field | Description |
|---|---|
| `app_id` | Unique identifier — used in JWT, Redis keys, metrics |
| `display_name` | Human-readable name for dashboards |
| `context_key_claims` | Ordered list of claim names that compose `contextKey` |
| `resources` | Map of resource name → URL template + TTL |
| `secret_arn` | AWS Secrets Manager ARN for per-app signing secret |
| `rate_limit` | Max requests/min on `/hydrate` for this app |
| `hydration_token_ttl` | Persistent JWT TTL (default 30d) |

### URL templates

Backend URLs are defined as templates. Claims from the Redis mapping are substituted at hydration time:

```
Template: "https://svc/users/{user_id}/profiles/{profile_id}"
Claims:   { user_id: "u1", profile_id: "pA" }
Resolved: "https://svc/users/u1/profiles/pA"
```

Apps without certain claims simply omit those placeholders:

```
Template: "https://svc/users/{user_id}/profile"
Claims:   { user_id: "u1" }
Resolved: "https://svc/users/u1/profile"
```

---

## TTL and Staleness Strategy

### Per-resource TTL

Each app defines TTLs per resource at registration time. The platform enforces them:

| Resource type | Suggested TTL | Rationale |
|---|---|---|
| Profile (name, email, avatar) | 4–12h | Stable, low sensitivity |
| Preferences | 2–4h | User-controlled, occasional changes |
| Permissions / access data | 5–15m | Changes must propagate reasonably fast |
| Dynamic resources / limits | 1–5m | High churn, accuracy matters |

### Invalidation strategies

**1. TTL expiry (baseline — always in place)**
Cache entries expire automatically. Next hydration request after expiry fetches fresh data.

**2. Re-hydrate on login (recommended)**
Every login triggers a fresh hydration, overwriting stale cache regardless of TTL. Sessions always start with current data.

**3. Event-driven invalidation (for high-churn resources)**
Backend publishes an invalidation event when data changes. Hydration service subscribes and deletes the specific cache key immediately.

```
Resource changed in backend
    → publish: { "app_id": "payments-app", "context_key": "u1:acc-99", "resource": "limits" }
    → hydration service deletes: payments-app:limits:u1:acc-99
    → next read: cache miss → re-hydrated
```

**4. Stale-while-revalidate (for smooth expiry)**
When TTL drops below 20% of original, trigger background re-hydration while still serving the cached value. Eliminates cold-miss latency spikes.

```
limits TTL = 5m → background re-hydrate when TTL < 1m remaining
caller receives cached value immediately
cache is refreshed before it expires
```

---

## Client SDK

App teams use the SDK — they never call platform APIs directly.

### Go SDK

```go
import "github.com/yourorg/hydrator-sdk/go"

// initialise once at startup
client := hydrator.NewClient(hydrator.Config{
    AppID:  "payments-app",
    Secret: secretFromAWS,
})

// at login — issue hydration token and set cookie
token, err := client.IssueToken(hydrator.Claims{
    "user_id":    userID,
    "account_id": accountID,
})
client.SetCookie(w, token)  // sets HttpOnly, Secure, SameSite, Path=/hydrate

// post-auth — read hydrated data
profile, err := client.GetData(contextKey, "profile")
limits,  err := client.GetData(contextKey, "limits")
```

### TypeScript SDK

```typescript
import { HydratorClient } from '@yourorg/hydrator-sdk'

const client = new HydratorClient({ appId: 'payments-app', secret })

// at login
const token = await client.issueToken({ userId, accountId })
res.cookie('hyd', token, { httpOnly: true, secure: true, sameSite: 'strict', path: '/hydrate' })

// post-auth
const profile = await client.getData(contextKey, 'profile')  // null on cache miss
const limits  = await client.getData(contextKey, 'limits')
```

SDK responsibilities:
- Signs and verifies JWTs using the app's secret
- Sets cookie with correct security flags
- Reads data from context-reader-service
- Returns `null` on cache miss — caller decides whether to trigger re-hydration or fall back to backend

---

## Complete Request Flow

### Pre-auth hydration (returning visit)

```
1. Browser visits app with persistent hyd cookie from previous session
2. App frontend calls POST /hydrate with cookie JWT
3. hydration-service:
     a. Verifies JWT signature + expiry
     b. Resolves hyd_token → { contextKey, claims } from Redis mapping
     c. Loads app config for appID
     d. Fans out parallel fetches to backend services using URL templates
     e. Writes results to Redis: appID:resource:contextKey (per-resource TTL)
     f. Returns 202 Accepted (no body)
4. User authenticates → session token issued
5. App reads context: GET /data/{contextKey}/profile → cache HIT → <1ms response
```

### Post-auth data read

```
1. Authenticated request arrives with session token
2. App extracts userID + profileID from session JWT directly (no lookup needed)
3. For full context: GET /context/{contextKey}?resources=profile,limits
4. context-reader-service:
     a. Verifies session token
     b. Reads from Redis: appID:profile:contextKey, appID:limits:contextKey
     c. Returns all available data + meta.source per resource
5. On cache miss: returns null — caller triggers re-hydration or falls back to backend
```

### Login (new session, any profile)

```
1. Auth service validates credentials
2. Fetches userID + profileID (or other claims)
3. Calls SDK: client.IssueToken({ user_id, profile_id })
     → generates contextKey = "u1:pA"
     → generates hyd_token = HMAC("u1:pA", secret)
     → stores Redis mapping: hyd:mapping:appID:hyd_token = { contextKey, claims }
     → signs JWT with appID + hyd_token + exp
4. Sets persistent hyd cookie (30d TTL)
5. Triggers POST /hydrate immediately — cache warm before first authenticated request
6. Issues session JWT: { user_id, profile_id, exp: 1h }
```

---

## Operational Model

### Secrets management

- Each app has its own secret in AWS Secrets Manager
- Secrets are fetched at service startup, held in memory, never written to disk or logs
- Rotating one app's secret does not affect other apps
- Secret rotation invalidates all persistent cookies for that app — users re-login once

### Rate limiting

`/hydrate` is internet-facing and unauthenticated — rate limiting is the primary defence against abuse and backend hammering.

Three layers of rate limiting applied in order:

**1. API Gateway — IP-based (outermost)**
Rejects flood traffic before it reaches the service. Configured at the gateway level (AWS API Gateway, CloudFront, nginx):
```
100 requests / IP / minute   → 429
```

**2. WAF rules — pattern-based**
Block known abuse patterns at the edge:
- Requests with malformed or oversized JWT payloads
- IPs exceeding burst thresholds (e.g. 20 req/sec)
- Known bad IP ranges via managed rule groups

**3. Service-level — per app token (innermost)**
Enforced inside hydration-service using Redis counters after JWT validation:
```
Key:   ratelimit:<appID>:<hyd_token_hash>:<minute-bucket>
Value: request count
TTL:   2 minutes
```

Prevents a single stolen token from flooding backend services even if it passes IP rate limiting (e.g. distributed attack from multiple IPs).

Exceeds any limit → `429 Too Many Requests`, no body.

### Observability

All metrics, logs, and traces are tagged with `app_id`:

```
hydration_latency_ms{app_id, resource}
cache_hit_rate{app_id, resource}
cache_miss_total{app_id, resource}
hydration_errors_total{app_id, resource}
```

### Ownership boundary

| Concern | Owner |
|---|---|
| hydration-service, context-reader-service, app-registry | Platform team |
| Redis cluster, AWS Secrets Manager | Platform team |
| Go/TypeScript SDK | Platform team |
| Backend service URLs and API contracts | App team |
| contextKey claim definitions | App team |
| TTL values per resource | App team |
| Calling SDK at login and data-read time | App team |

---

## Network Topology

```
                        ┌─────────────────────────────┐
  internet          ───►│  API Gateway / Auth Layer    │
                        └──────────┬──────────────────┘
                                   │
              ┌────────────────────┼────────────────────┐
              │                                         │
              ▼                                         ▼
  ┌───────────────────────┐             ┌───────────────────────┐
  │   hydration-service   │             │  context-reader-svc   │
  │   POST /hydrate       │             │  GET /data            │
  │   (pre-auth)          │             │  GET /context         │
  │   no session needed   │             │  (session required)   │
  └──────────┬────────────┘             └──────────┬────────────┘
             │ writes                               │ reads only
             └──────────────┬───────────────────────┘
                            ▼
                    ┌───────────────┐
                    │     Redis     │
                    │  (namespaced) │
                    └───────────────┘
             │ fetches
             ▼
  ┌─────────────────────────┐
  │  App backend services   │
  │  (internal network)     │
  │  profile, preferences,  │
  │  permissions, resources │
  └─────────────────────────┘
```

- `hydration-service` has read/write access to Redis and outbound access to backend services
- `context-reader-service` has read-only access to Redis — no backend service access
- Backend services are on the internal network only — never publicly accessible
- Compromise of `context-reader-service` yields Redis read access only — no path to backends

---

## Onboarding a New App Team

```
1.  App team calls POST /platform/apps/register
          with app_id, backend URLs, resource definitions, TTLs
          ↓
2.  Platform returns secret_arn + endpoints
          ↓
3.  App team adds SDK dependency (Go or TypeScript)
          ↓
4.  App team fetches secret from AWS Secrets Manager at startup
          ↓
5.  At login:
          client.IssueToken(claims) → generates hyd_token, stores Redis mapping
          client.SetCookie(w, token) → sets persistent hyd cookie
          POST /hydrate → warms cache immediately
          ↓
6.  Post-auth:
          client.GetData(contextKey, "profile") → reads from Redis
          null on miss → app decides: re-hydrate or fall back to backend
```

Zero platform team involvement after step 2.
