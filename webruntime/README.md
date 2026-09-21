# Unified Web Runtime — POC

A runnable proof-of-concept for the target architecture in
`RFC_Unified_Web_Runtime v4.docx`: one versioned Web Runtime, one Web
Runtime Config Service, channel teams build once. Four containers, wired
via `docker compose`.

## Run it

```
docker compose up --build
```

Then open **http://localhost:8081** and log in as one of the demo users.
You'll be redirected through the Web Runtime Service, which renders
`index.html` with the tenant's manifest-driven nav and lazy-loads an
"experience" module from the CDAaS mock.

To walk through WRCS's versioning, blue-green promotion, rollback, and diff
capabilities via the API:

```
./scripts/demo.sh   # requires curl + jq
```

## Services

| Service | Port | Role |
|---|---|---|
| `oidc-shell` | 8081 | Mints a demo JWT, redirects to the Web Runtime Service. Auth only — never serves HTML (RFC goal G4). |
| `web-runtime-service` | 8080 | Go BFF: validates JWT, fetches manifest from WRCS (cached), evaluates access control via embedded OPA, renders `index.html` + `__PLATFORM_STATE__`. |
| `wrcs` | 8091 | Web Runtime Config Service: versioned immutable manifests per tenant/env, blue-green slots, rollback, diff, webhook event emission. |
| `cdaas-mock` | 8092 | Static file server standing in for CDAaS — serves the "experience" JS modules the browser lazy-loads. |

Request flow mirrors the RFC's Target Architecture: browser → OIDC Shell
(JWT) → Web Runtime Service (manifest + OPA + render) → browser loads the
Web Runtime shell → shell dynamic-`import()`s the active experience from
CDAaS.

## What's simplified for the POC

- **Auth**: hand-rolled HS256 JWT with a shared demo secret, not real
  OIDC/JWKS.
- **Cache**: RFC's "L1 → Redis → origin" collapses to "L1 → origin"; no
  Redis container. The L1 cache is invalidated by a webhook from WRCS on
  publish/promote/rollback, so this still demonstrates capability #6
  (event emission) without a message broker.
- **Module delivery**: CDAaS mock serves plain ES modules loaded via
  dynamic `import()`, not real Webpack Module Federation `remoteEntry.js`.
- **Registry**: no CDAaS auto-patch of `moduleVersion` pointers — the
  manifest's `moduleVersion` is set manually on publish.
- **Multi-tenancy, not CBx/JPMM migration**: two generic tenants
  (`retail-checkout`, `wealth-dashboard`) demonstrate config isolation
  (capability #1). JPMM-specific integrations (Profile, Entitlements, OCS)
  are Phase 5 in the RFC and require separate OCS co-design — not built
  here.
- **Config authoring**: no DE Console UI; manifests are published via
  WRCS's REST API (`curl`), matching capability #8 ("CLI, CI/CD pipelines...
  publish manifests via REST API").

## Manifest schema

`manifests/*.json` are seed manifests matching the RFC's abbreviated
schema — `tenantId`, `env`, `runtime.version`, `shell.nav[].experience`,
`shell.auth.roles`. WRCS publishes them as version 1 (stable + candidate)
on first boot; the `wrcs-data` volume persists everything published after
that across restarts.

## WRCS API

```
POST /manifests/{tenant}/{env}/publish     body: manifest JSON  -> new version, sets candidate
POST /manifests/{tenant}/{env}/promote                          -> candidate becomes stable
POST /manifests/{tenant}/{env}/rollback    body: {"version": n, "slot": "stable"|"candidate"}
GET  /manifests/{tenant}/{env}/{slot}      slot = stable | candidate
GET  /manifests/{tenant}/{env}/versions
GET  /manifests/{tenant}/{env}/diff?from=1&to=2
```

## Proving access control is real (RFC goal G7)

From the OIDC Shell homepage, use "Log in with the wrong role" — you'll get
a genuine `403` from the Web Runtime Service, produced by an actual OPA
`rego` evaluation (`services/web-runtime-service/opa/policy.rego`) against
the manifest's `shell.auth.roles`, not a hardcoded check.
