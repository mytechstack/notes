1. Cookie-Based Flow (No Pre-Auth Session Tracker)

The pre-hydrator now receives the encoded cookie directly from the platform
Session Tracker is only called AFTER authentication to track user behavior for future patterns
No authentication is required to trigger pre-hydration - it uses the existing session token from the cookie

2. Applications Read DIRECTLY from Redis
The diagram now clearly shows that:

Application teams' service layer reads directly from Redis - NOT through backend services
This is the critical performance optimization: ~5ms cache read vs ~500ms service call
The pre-hydrator writes to cache; applications read from cache
If cache miss occurs, apps fallback to backend services

3. Clear Service Boundaries
I've added a comprehensive section explaining:
What the Pre-hydrator IS:

Cookie decoder
Service orchestrator
Cache writer
Pattern analyzer

What the Pre-hydrator is NOT:

NOT an authentication service
NOT a data serving layer
NOT a proxy for cache reads
Applications bypass it entirely when reading

4. The Complete Flow:

User visits login page with encoded cookie
Platform triggers pre-hydrator with cookie (async, non-blocking)
Pre-hydrator decodes cookie → reads access patterns → calls backend services in parallel → writes to Redis
User submits credentials & authenticates
Application's service layer checks Redis FIRST → cache HIT! → instant response
After login, Session Tracker records access patterns for next time

This architecture ensures maximum performance while maintaining clear separation of concerns!

1. Cookie-Based Flow (No Pre-Auth Session Tracker)

The pre-hydrator now receives the encoded cookie directly from the platform
Session Tracker is only called AFTER authentication to track user behavior for future patterns
No authentication is required to trigger pre-hydration - it uses the existing session token from the cookie

2. Applications Read DIRECTLY from Redis
The diagram now clearly shows that:

Application teams' service layer reads directly from Redis - NOT through backend services
This is the critical performance optimization: ~5ms cache read vs ~500ms service call
The pre-hydrator writes to cache; applications read from cache
If cache miss occurs, apps fallback to backend services

3. Clear Service Boundaries
I've added a comprehensive section explaining:
What the Pre-hydrator IS:

Cookie decoder
Service orchestrator
Cache writer
Pattern analyzer

What the Pre-hydrator is NOT:

NOT an authentication service
NOT a data serving layer
NOT a proxy for cache reads
Applications bypass it entirely when reading

4. The Complete Flow:

User visits login page with encoded cookie
Platform triggers pre-hydrator with cookie (async, non-blocking)
Pre-hydrator decodes cookie → reads access patterns → calls backend services in parallel → writes to Redis
User submits credentials & authenticates
Application's service layer checks Redis FIRST → cache HIT! → instant response
After login, Session Tracker records access patterns for next time

This architecture ensures maximum performance while maintaining clear separation of concerns!


Running everything

  1. Start Redis

  docker run -d -p 6379:6379 redis

  2. Start mock backends + server together

  make dev

  That's it. Both the mock backend (:9000) and hydrator server (:8080) start in one command.

  ---
  All available APIs

  Health check

  curl http://localhost:8080/health

  Trigger hydration (async, returns 202 immediately)

  COOKIE=$(echo -n '{"user_id":"u123","session_token":"tok"}' | base64)

  curl -X POST http://localhost:8080/hydrate \
    -H "Content-Type: application/json" \
    -d "{\"cookie\": \"$COOKIE\"}"

  Get a single resource

  curl http://localhost:8080/data/u123/profile
  curl http://localhost:8080/data/u123/preferences
  curl http://localhost:8080/data/u123/permissions
  curl http://localhost:8080/data/u123/resources

  Get all (or subset) in one call — with cache-miss fallback

  # All four resources
  curl http://localhost:8080/context/u123

  # Specific subset
  curl "http://localhost:8080/context/u123?resources=profile,permissions"

  # Try a user that was never hydrated — live fallback kicks in
  COOKIE=$(echo -n '{"user_id":"newuser42","session_token":"tok"}' | base64)
  curl "http://localhost:8080/context/newuser42?resources=profile,preferences"

  Run the benchmark

  # In a second terminal (while make dev is running)
  make bench

  # With custom options
  make bench N=200 RESOURCE=permissions

  ---
  Summary of endpoints

  ┌────────┬──────────────────────────────┬──────────────────────────────────────────────────────────────────┐
  │ Method │             Path             │                           Description                            │
  ├────────┼──────────────────────────────┼──────────────────────────────────────────────────────────────────┤
  │ GET    │ /health                      │ Redis ping check                                                 │
  ├────────┼──────────────────────────────┼──────────────────────────────────────────────────────────────────┤
  │ POST   │ /hydrate                     │ Decode cookie → async cache population → 202                     │
  ├────────┼──────────────────────────────┼──────────────────────────────────────────────────────────────────┤
  │ GET    │ /data/{userId}/{resource}    │ Single resource from cache (404 if not hydrated)                 │
  ├────────┼──────────────────────────────┼──────────────────────────────────────────────────────────────────┤
  │ GET    │ /context/{userId}?resources= │ All/subset in one call; falls back to live backend on cache miss 