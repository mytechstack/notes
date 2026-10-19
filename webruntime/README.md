# Unified Web Runtime — POC

A runnable proof-of-concept for the target architecture in
`RFC_Unified_Web_Runtime v4.docx`: one versioned Web Runtime, one config
surface, channel teams build once — with **each manifest built once and
promoted unchanged through dev → sit → perf → uat → prod**, and blue-green,
canary and instant rollback inside every environment, all driven by
immutable manifests in S3 and one pointer file per tenant per environment.

## Run it

```
docker compose up --build
```

| Open | What |
|---|---|
| **http://localhost:8093** | Control plane UI: configure pages → publish (lands in dev) → promote through environments → canary / rollback per environment → see which users get which version |
| **http://localhost:8081** | Log in (pick environment, user id, or preview the candidate) and see the app |

The whole loop as a script: `./scripts/demo.sh` (requires `curl`, `jq`).

## Services

| Service | Port | Role |
|---|---|---|
| `control-plane` | 8093 | DE Console stand-in: owns tenant **config** (one per tenant, not per environment) in its own database, and the module catalog. Emits `config.changed` to WCR. Serves the UI. |
| `wrcs` (WCR) | 8091 | Web Composition Registry: generates ONE environment-agnostic manifest per config revision, stores it in S3 as an **immutable version**, owns each environment's **pointer** (stable / candidate / canary) and the **promotion gates** between environments. Not on the request path. |
| `s3` | 9000 | S3-compatible storage (SeaweedFS). Bucket `web-manifests`; anonymous read-only, WCR's key read/write (`infra/s3/s3.json`). |
| `web-runtime-service` | 8080 | Go BFF: validates JWT, reads the environment's pointer + manifest **directly from S3**, binds the manifest to that environment's hosts (`infra/envs.json`), picks a version per user, OPA access check, renders `index.html` + `__PLATFORM_STATE__`. |
| `oidc-shell` | 8081 | Mints a demo JWT (any user id) and redirects to the Web Runtime Service. Its login page lists every tenant from the control plane with one-click links per environment. Auth only (RFC goal G4). |
| `cdaas-mock` | 8092 | Serves the experience JS modules the browser lazy-loads. |

## How release works

```
control plane ──config.changed──▶ WCR ──builds ONCE──▶ S3  manifests/{tenant}/v{n}.json     immutable, no hosts
 (config DB, per tenant)            │                       releases/{tenant}/v{n}.json      where it has been stable
                                    └──writes──────────▶    pointers/{tenant}/{env}.json     one per environment

     dev ──promote──▶ sit ──promote──▶ perf ──promote──▶ uat ──promote──▶ prod
  (automatic)      each hop: lands as CANDIDATE; canary / promote-to-stable inside the env

Web Runtime Service, per request:  ?env=…  → that environment's pointer + binding (hosts)
                                    ?slot=candidate        → candidate      ("preview")
                                    bucket(user) < percent → canary version ("canary")
                                    otherwise              → stable         ("stable")
```

| Action | API (WCR) | What changes |
|---|---|---|
| Save config | — (event) | WCR builds `v(N+1)` once; **dev** stable = v(N+1). Other environments untouched |
| Promote to next environment | `POST /manifests/{t}/promote {version, from, to}` | `to`'s **candidate** = version. Refused (409) unless `to` is the next environment after `from` **and** the version has been stable in `from` |
| Canary 10 / 25 / 50 % | `POST /manifests/{t}/{env}/canary {percent}` | that environment's canary |
| Promote candidate → stable | `POST /manifests/{t}/{env}/promote` | that environment's stable (blue-green switch) |
| Rollback ("Make stable in env") | `POST /manifests/{t}/{env}/rollback {version}` | that environment's stable — nothing rebuilt; other environments untouched |

- **Build once, promote the same file.** Manifests carry module **paths** (`/home/App.js`), never hosts,
  so v5 in prod is byte-for-byte the v5 UAT signed off (same S3 object, same ETag). Hosts come from
  each environment's **binding** in `infra/envs.json`; the Web Runtime Service fills them in at request
  time (`bind` in `services/web-runtime-service/envs.go`). In production each environment's service
  carries only its own binding; the POC runs one service for all five and picks by `?env=`.
- **Release record.** `releases/{tenant}/v{n}.json` lists the environments where the version has been
  stable plus a history; the promotion gate reads it.
- **Sticky, monotonic canary.** `bucket = fnv32a(tenant + ":" + userId) % 100`; the same user always
  lands in the same bucket, and raising the percentage only adds users. `GET /debug/assign` on the Web
  Runtime Service runs this exact code for any list of users and any environment.
- **Fast propagation, static stability.** Pointer writes ping the Web Runtime Service (5 s poll as
  fallback); manifests are cached forever; if S3 is down the last good copy keeps serving.

## Tenants and modules

**Modules are independent of tenants.** The module catalog
(`services/control-plane/seed/modules.json`) lists each module once with all
its released versions; tenants *reference* modules from their pages and pin an
exact version:

```json
"home": { "scope": "home", "latest": "1.1.0",
          "versions": { "1.0.0": { "path": "/home/App.js", … },
                        "1.1.0": { "path": "/home/1.1.0/App.js", … } } }
```

- A page stores `module` + `moduleVersion`; leaving the version empty pins the
  module's `latest` at save time, so stored config is always exact.
- Different tenants (and the same tenant in different environments) can run
  different versions: e.g. dev on `home@1.1.0` (adds a **Runway** card) while
  prod still runs `home@1.0.0` — until that manifest is promoted.
- The console shows a version picker per page and an "↑ x.y.z available" hint.
- Module behaviour per tenant comes from data/config (`ctx.tenant`,
  `dataPath`), never from per-tenant forks of a module.

**Creating a tenant** (console **+ New tenant**, or the API):

```
POST /api/tenants  {"tenantId":"acme-bank","displayName":"Acme Bank","template":"banking-standard","role":""}
```

- Templates live in `services/control-plane/seed/templates/` (`banking-standard`,
  `checkout-basic`, `starter`); pages are pinned to each module's latest version.
- The role defaults to `{tenantId}-user` (what the OIDC shell must issue).
- WCR builds v1 and puts it live in **dev only**. Other environments answer
  "Not deployed in {env} yet" until the version is promoted there — the same
  gates as any change. Canary is refused in an environment with nothing
  stable yet (there is no baseline to compare against).
- Seeded tenants (`startup-banking`, `retail-checkout`) model tenants that
  already exist, so their first build is marked `bootstrap` and goes live in
  every environment.

## Experience frame (header, navigation, footer)

The frame around every page is built from **frame modules** — versioned
modules in the same catalog as page experiences, marked `"kind": "frame"` with a
`slot` (`header`, `nav`, `footer`) and a `settings` schema:

| Module | Settings |
|---|---|
| `frame-header` | title, logoUrl, showUser |
| `frame-nav` | position (`left` / `top`), showIcons |
| `frame-footer` | text, links (`[{label, url}]`) |

- **Config:** each tenant's config has a `frame` section — `theme`
  (`primary`, `headerBg`) plus, per slot, `module`, `moduleVersion` and
  `settings`. The console's **Experience frame** card edits it (module and
  version per slot, a form generated from the schema, colour pickers) and it is
  saved by the same **Save & publish** as pages. The footer may be empty.
- **Validation:** a slot only accepts a frame module for that slot, pages only
  accept page experiences, settings must match their type, URLs must be
  `http(s)://`, colours `#rrggbb`.
- **Manifest:** WCR writes `shell.frame` (`theme`, and per slot the module's
  `path`, version and settings). The frame is therefore built once and promoted
  dev → prod, canaried and rolled back exactly like pages.
- **Runtime:** the Web Runtime Service renders plain fallbacks for each slot (title,
  nav links, footer) and applies the nav position and theme on the server, so nothing
  jumps. `runtime.js` then loads the frame modules from CDAaS and calls
  `mount(el, ctx)` with `{tenant, env, user, version, selectedBy, settings, theme,
  nav: {items, activeId, navigate(id)}}`; the nav module returns
  `{setActive(id)}`. If a frame module fails to load, its fallback stays and the
  page keeps working.

## What's simplified for the POC

- **Auth**: hand-rolled HS256 JWT with a shared demo secret; control plane and WCR APIs are
  unauthenticated.
- **Environments**: all five share one bucket, one WCR and one Web Runtime Service. In production,
  prod usually has its own account/bucket: promotion into prod copies the immutable object across
  (verifying its hash) through a narrow promote-only role, and prod's runtime reads only prod's bucket.
- **Approvals**: the gate checks "stable in the previous environment" only; real promotion into
  uat/prod would also require sign-off and attached test results in the release record.
- **Pointer writes**: serialized by a mutex inside WCR. In production use S3 conditional writes
  (`If-Match` on the ETag) so concurrent promotes can't overwrite each other, and enable bucket
  versioning on `pointers/` for an audit trail.
- **Delivery**: the Web Runtime Service reads S3 directly. In production put a CDN in front:
  long TTL on `manifests/*` (immutable), seconds on `pointers/*`, and use S3 event
  notifications instead of a direct webhook.
- **Events**: `config.changed` is an HTTP POST from a file-backed outbox, not a broker.
- **Module delivery**: plain ES modules via dynamic `import()`, not Module Federation.
- **Registry**: the module catalog is a read-only seed file
  (`services/control-plane/seed/modules.json`) — no CI publishing of new
  module versions and no compatibility checks yet.
- **Tenants**: no tenant registry beyond the control plane's config folders,
  no deletion, and the mock OIDC shell accepts any tenant/role.
- **Personalization** (OCS / Resolver in the real system) isn't modelled beyond the
  server-fetched `ssrData` for the Home dashboard.

## APIs

Control plane (`:8093`):
```
GET  /api/tenants   GET /api/modules   GET /api/templates
POST /api/tenants   {"tenantId","displayName","template","role"} -> revision 1, live in dev only
GET  /api/configs/{tenant}                  latest config + revision
PUT  /api/configs/{tenant}    body: config  -> validates, new revision, emits config.changed
GET  /api/configs/{tenant}/revisions/{n}    snapshot WCR pulls to generate
GET  /api/events                            outbox
```

WCR (`:8091`):
```
POST /events                               config.changed -> 202, build async, deploy to dev
GET  /manifests/{tenant}/versions          all versions (+ stableIn) and every environment's pointer
GET  /manifests/{tenant}/versions/{n}      one immutable version
GET  /manifests/{tenant}/diff?from=&to=
POST /manifests/{tenant}/promote           {"version", "from", "to"}  -> next env's candidate (gated)
POST /manifests/{tenant}/{env}/canary      {"percent": 0-100}
POST /manifests/{tenant}/{env}/promote     candidate -> stable in that env
POST /manifests/{tenant}/{env}/rollback    {"version": n, "slot": "stable"|"candidate"}
GET  /manifests/{tenant}/{env}/{slot}      the manifest a slot points at
GET  /events   GET /tenants
```

Web Runtime Service (`:8080`):
```
GET  /?token=…&env=dev|sit|perf|uat|prod[&slot=candidate]   render index.html
GET  /debug/assign?tenant=&env=&users=a,b     which version each user gets (same code path)
POST /internal/cache-invalidate               WCR webhook: refresh the pointer now
```

S3 (`:9000`, anonymous read): `GET /web-manifests/pointers/{tenant}/{env}.json`,
`GET /web-manifests/manifests/{tenant}/v{n}.json`, `GET /web-manifests/releases/{tenant}/v{n}.json`.

## Proving access control is real (RFC goal G7)

Log in with role `wrong-role` — the Web Runtime Service returns a genuine `403` from an OPA
`rego` evaluation (`services/web-runtime-service/opa/policy.rego`) against the manifest's
`shell.auth.roles`.
