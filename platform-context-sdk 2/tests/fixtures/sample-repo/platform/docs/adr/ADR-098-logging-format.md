# ADR-098: Structured logging format

## Status
Accepted

## Context
Services were emitting inconsistent log formats, making cross-service
log correlation difficult.

## Decision
All services must emit JSON-structured logs with a required `trace_id`
field. Use the platform's `logging-lib` package.

## Consequences
No app-level string logging. All log calls go through `logging-lib`.
