# ADR-142: Rate limiting at the gateway layer

## Status
Accepted

## Context
auth-service needed rate limiting to prevent credential-stuffing attacks.
We considered app-level rate limiting vs. gateway-level.

## Decision
Rate limiting belongs at the API gateway layer, not in application code.
Use the token_bucket strategy — it handles bursty legitimate traffic better
than sliding_window for our use cases.

## Consequences
Any service needing rate limiting should add a `rate_limit` block to its
`gateway.yaml` rather than implementing limiting in app code. See
auth-service/gateway.yaml for a working example.
