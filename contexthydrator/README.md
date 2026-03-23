# Context Hydrator

A cache pre-hydration service that eagerly fetches user context (profile, preferences, permissions, resources) from upstream backend services and stores it in Redis. Downstream consumers can then read from the cache at sub-millisecond latency instead of hitting the backends on every request.

## Prerequisites

- Go 1.21+
- Redis 6+

## Setup

```bash
git clone <repo-url>
cd contexthydrator

# Copy and edit the example env file
cp .env.example .env
```

Edit `.env` to point `PROFILE_SERVICE_URL`, `PREFERENCES_SERVICE_URL`, `PERMISSIONS_SERVICE_URL`, and `RESOURCES_SERVICE_URL` at your real backend services. When using the built-in mock backend (see below), they all point to `http://localhost:9000` — which is already the default in `.env.example`.

## Running with Docker

The fastest way to get everything running — no local Go or Redis install needed.

```bash
cp .env.example .env   # only needed once
make docker-up
```

This starts three containers: `redis`, `mockbackend`, and `hydrator`. The hydrator is available at `http://localhost:8080`.

```bash
make docker-down       # stop and remove containers
make docker-build      # build the image only
```

The `docker-compose.yml` overrides `REDIS_ADDR` and the service URLs to use container hostnames automatically, so `.env` values for those are ignored when running via Compose.

---

## Running the Project

### Development (mock backend + hydrator together)

```bash
make dev
```

Builds both binaries, starts the mock backend on port `9000`, and starts the hydrator on port `8080`. The mock backend simulates upstream services with configurable latency. Stops both on `Ctrl-C`.

### Server only (with real backend services)

```bash
make run
```

Builds and runs only the hydrator server. Use this when `PROFILE_SERVICE_URL` etc. point at real services.

### Build binaries

```bash
make build        # bin/server
make build-mock   # bin/mockbackend
make build-bench  # bin/benchmark
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET/HEAD` | `/health` | Liveness check — returns `200 OK` |
| `POST` | `/hydrate` | Trigger async hydration for a user. Body: `{"cookie": "<base64-encoded-json>"}`. Returns `202 Accepted`. |
| `GET/HEAD` | `/data/{userId}/{resource}` | Read a single cached resource (`profile`, `preferences`, `permissions`, `resources`). Returns `404` on cache miss. |
| `GET/HEAD` | `/context/{userId}` | Read all four cached resources for a user in one response. |

## Running Benchmarks

The benchmark compares the **cold path** (direct call to the backend service) against the **hot path** (Redis cache read after hydration).

**1. Start the dev environment in one terminal:**

```bash
make dev
```

**2. In a second terminal, run the benchmark:**

```bash
make bench
```

Defaults: 100 requests per path, `profile` resource.

**Custom parameters:**

```bash
make bench N=200 RESOURCE=permissions
```

| Parameter | Default | Description |
|-----------|---------|-------------|
| `N` | `100` | Number of requests per path |
| `RESOURCE` | `profile` | Resource to benchmark (`profile`, `preferences`, `permissions`, `resources`) |

**Sample output:**

```
──────────────────────────────────────────────────────────────
               mean       p50       p95       p99       max
──────────────────────────────────────────────────────────────
  cold (backend)   52.10ms   51.80ms   58.20ms   63.40ms   71.20ms
  hot  (cache)      0.62ms    0.59ms    0.91ms    1.10ms    2.30ms
──────────────────────────────────────────────────────────────
  improvement        84x faster  88x faster  64x faster  58x faster
──────────────────────────────────────────────────────────────
```

- **cold (backend)** — latency of direct calls to the upstream backend (includes simulated network delay)
- **hot (cache)** — latency of cache reads from Redis after hydration
- **p50/p95/p99** — latency percentiles across all requests
- **improvement** — speedup factor at each percentile

## Testing

```bash
make test                 # unit tests
make test-integration     # integration tests (requires Redis)
make lint                 # go vet
```

## Configuration

All settings are read from environment variables (or a `.env` file in the project root).

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP listen port |
| `READ_TIMEOUT` | `5s` | HTTP read timeout |
| `WRITE_TIMEOUT` | `10s` | HTTP write timeout |
| `LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | `json` | Log format: `json` or `text` |
| `REDIS_ADDR` | `localhost:6379` | Redis address |
| `REDIS_PASSWORD` | _(empty)_ | Redis password |
| `REDIS_DB` | `0` | Redis database number |
| `PROFILE_SERVICE_URL` | `http://localhost:9000` | Upstream profile service URL |
| `PREFERENCES_SERVICE_URL` | `http://localhost:9000` | Upstream preferences service URL |
| `PERMISSIONS_SERVICE_URL` | `http://localhost:9000` | Upstream permissions service URL |
| `RESOURCES_SERVICE_URL` | `http://localhost:9000` | Upstream resources service URL |
| `BACKEND_TIMEOUT_SECS` | `4` | Timeout (seconds) for all backend calls |
| `COOKIE_ENCODING` | `base64json` | Cookie decoding mode: `base64json` or `jwt` |
| `COOKIE_SECRET` | `change-me` | Secret key (required when `COOKIE_ENCODING=jwt`) |
