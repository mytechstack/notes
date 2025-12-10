# ADR: Self-Hosted esm.sh-like Transformation Service vs Static Server

Status: Proposed

## Decision Summary

Chosen option: Deploy a self-hosted instance of an esm.sh-like module transformation service.

Alternative: Host pre-transformed or pre-bundled dependencies on a basic static file server (S3/Blob Storage).

Rationale: A self-hosted transformation service preserves the zero-config ESM import experience, automatic optimizations (tree-shaking, targeting, source maps), and private-package access while meeting internal security and compliance requirements that a static server cannot.

---

## Context

We are building a converged microfrontend platform that serves JavaScript modules to multiple application teams. This decision affects:

- Developer velocity and DX
- Module loading performance
- Dependency/version management
- Infrastructure and maintenance burden
- Time-to-production for new modules and experiments

Critical requirements include developer velocity, dependency management at scale, version flexibility, minimized bundle size, and limited platform team bandwidth.

---

## Decision Drivers

- Developer Velocity: quick iteration and low friction for experimenting with packages
- Shared Dependencies: many teams share libraries (React, MUI, utilities)
- Version Flexibility: concurrent support for different versions on the same page
- Bundle Size: avoid duplicate code and enable tree-shaking
- Maintenance: keep platform maintenance overhead low
- Compliance: support air-gapped/offline builds and private package access

---

## Analysis and Key Capabilities

A transformation service provides runtime (or on-demand) module transformation and optimization features that a static server cannot provide without per-application build steps.

Capabilities that benefit application teams:

- On-demand CJS → ESM transformation
- Automatic tree-shaking / export filtering
- Browser-targeted bundles and optional polyfills
- Dev builds and source maps for easier debugging
- Support for CSS-in-JS, workers, and other assets via transformation rules
- Integration with private/internal package registries

Comparison summary:

- Static server: serves pre-built files. Requires each app to maintain bundlers, build pipelines, and coordination for shared deps.
- Self-hosted transformer: centralizes transformation logic, enforces org presets, and enables zero-config imports from NPM and private registries.

---

## Use Cases (Concise)

1. NPM package consumption
   - Problem (static): each app bundles deps → duplicate downloads
   - With transform service: apps import packages directly, browser caches shared modules.

2. Dependency version conflicts
   - Problem (static): force single version or bundle duplicates (runtime issues)
   - With transform service: modules can import pinned versions and run concurrently.

3. Shared dependency optimization
   - Automatic dedupe and tree-shaking reduces total payload across many modules.

4. Developer velocity
   - Experiment with new packages by editing imports; no build pipeline changes required.

5. TypeScript support
   - Service can expose types mapped to runtime versions (e.g., via headers), reducing type drift.

6. Build pipeline maintenance
   - Centralized transformation simplifies platform maintenance vs. many per-app build configs.

---

## Risks and Mitigations

Risk: Operational overhead (maintenance, patching, scaling)
- Mitigation: Automate CI/CD (Docker/K8s), monitoring, and patch pipelines.

Risk: Resource consumption during on-demand transforms
- Mitigation: Use a durable cache (object storage + Redis) to avoid repeated transformations; pre-warm critical packages.

Risk: Time-to-feature for new JS/TS language features
- Mitigation: Rely on upstream OSS for feature updates; focus internal engineering on private registry integration and security.

Risk: External dependency availability
- Mitigation: Self-hosted instance paired with internal CDN and fallback static cache for resiliency.

---

## Trade-offs

When a static server is preferable:

- Proprietary/private packages that cannot be exposed via a CDN without additional controls.
- Environments that prohibit external network access and cannot allow any transform pipeline.
- Very large custom bundles where transformation adds complexity without benefit.

When a transformation service is preferable:

- Heavy NPM dependency use across many modules
- Multiple independent teams requiring version flexibility
- Fast iteration and experimentation workflows
- Limited platform team bandwidth to maintain per-app build pipelines

---

## Recommendation

Adopt a hybrid approach:

- Use the self-hosted transformation service for public NPM packages and any package that benefits from on-demand optimization.
- Use a static server (internal CDN) for proprietary code and internal-only packages where stricter controls are required.
- Generate import maps that combine both sources and provide a fallback path to internal cached bundles if the transformer is unavailable.

Implementation steps:

1. Deploy a self-hosted transformer integrated with internal registry access.
2. Configure persistent caching (object storage + Redis) and an internal CDN edge.
3. Update platform import-map generator to include esm-like URLs for public packages and internal CDN URLs for proprietary packages.
4. Phase migration: new modules opt-in; incrementally convert existing modules.

---

## Migration Path

- Phase 1: Enable new modules to import from the transformer; implement import-map generation.
- Phase 2: Migrate high-value modules (those with large shared deps) to use transformer imports.
- Phase 3: Backfill remaining modules as needed, remove redundant bundling steps.

---

## Success Metrics (6 months)

- Average module bundle size: target < 100 KB (baseline ~400 KB)
- Time to add new NPM dependency: target < 1 minute (baseline ~25 minutes)
- Platform build pipeline maintenance hours: target < 5 hrs/week (baseline ~40 hrs/week)
- Dependency version conflicts: target < 1 per quarter (baseline ~8 per quarter)
- Developer satisfaction: target > 8/10

---

## Consequences

Positive:
- Faster developer iteration and experimentation
- Lower platform maintenance burden
- Reduced bandwidth and improved load performance
- Unified developer experience for public and private packages (when integrated)

Negative:
- Increased operational responsibility to run the transformer
- Need to support private package access and secure registry integration
- Potential reliance on the transformer's availability (mitigated with caching/fallback)

---

## References

- esm.sh — https://esm.sh/
- Import maps specification — https://github.com/WICG/import-maps
- Bundle size analysis and benchmarking tools (Bundlephobia)

---

## Appendix: Short Examples

Importing a package directly (transformer):

import React from "https://internal-esm.example.com/react@18.2.0";

Serving proprietary module from internal CDN:

import { AuthService } from "https://cdn.company.com/modules/auth-service@2.1.0";

