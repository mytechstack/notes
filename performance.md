Recommendation: Keep Them Separate, But Measure Both
You should maintain separate baselines but measure them together in realistic scenarios. Here's why and how:
Why Separate Baselines Matter
Platform baseline (in isolation):

Measures the overhead your platform introduces (orchestration, networking, observability, security layers, etc.)
Gives you a clean metric to track platform improvements/regressions over time
Helps you answer: "How much does our platform cost in terms of performance?"
Essential for capacity planning and pricing models

Application performance budgets (on your platform):

Each app team gets a clear target for their app's performance
Prevents the "blame game" when issues arise
Allows teams to optimize independently
Enables you to set tiered service levels (e.g., premium apps get more resources)

Practical Strategy
1. Establish Platform Baseline

Deploy a reference application (simple, well-understood workload like a "hello world" service or standard CRUD app)
Measure key metrics: latency (p50, p95, p99), throughput, resource utilization, cold start times
Test at various scales to understand platform overhead curves
Document the "tax" your platform adds vs bare-metal/VM performance

2. Define Application Performance Budgets

Create budget templates based on app tier (e.g., latency-critical, batch processing, standard web)
Budget = Total allowable performance - Platform baseline
Example: If you need p95 latency <200ms and platform adds 50ms, apps get 150ms budget
Include resource budgets (CPU, memory, I/O) not just latency

3. Continuous Measurement

Test platform + real apps together in staging/production-like environments
Use synthetic monitoring to catch platform regressions
Track "budget compliance" per application team
Alert when platform baseline degrades OR apps exceed budgets

Key Metrics to Track Separately
Platform metrics:

Ingress/egress latency, service mesh overhead, DNS resolution time, autoscaling response time, observability overhead, authentication/authorization latency

Application metrics:

Business logic execution time, database query performance, external API calls, cache hit rates, application-level resource usage

Governance Model
Create a performance SLA that includes:

Platform performance guarantees (what you promise)
Application performance expectations (what teams must meet)
Clear ownership: platform team owns infrastructure performance, app teams own application code performance
Regular performance reviews with each application team

This separation gives you accountability, clarity, and the ability to optimize each layer independently while ensuring the combined system meets business requirements.