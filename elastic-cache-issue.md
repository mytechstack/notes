Immediate Fallback Strategies
Graceful degradation - When Redis is unavailable, catch the connection errors and either:

Allow users to continue as "anonymous" with limited functionality
Fall back to stateless authentication (like JWT tokens if you also use them)
Show a maintenance message but don't crash

Circuit breaker pattern - Detect when Redis is down and stop attempting connections for a period, preventing cascade failures and reducing load during recovery.
Architectural Improvements
Redis replication - Set up Redis with replicas (master-replica or Redis Sentinel). If the primary fails, a replica can be promoted automatically. This handles most common failure scenarios.
Redis Cluster - For higher availability, use Redis Cluster which provides automatic failover and data sharding across multiple nodes.
Backup session store - Implement a secondary storage layer:

Write sessions to both Redis (primary) and a database (backup)
On Redis failure, read from the database
This adds latency but ensures availability

Sticky sessions with local cache - If using multiple application servers, combine load balancer sticky sessions with in-memory caching on each server. Users stay on the same server, which caches their session locally and syncs with Redis when available.
Code-Level Resilience
Implement retry logic with exponential backoff, set appropriate timeouts for Redis operations (don't let them hang indefinitely), and always wrap Redis calls in try-catch blocks with sensible fallback behavior.
Monitoring & Alerting
Set up health checks for Redis, monitor connection pool exhaustion, and create alerts for increased error rates so you can respond quickly.
Best Approach?
For most applications, I'd recommend starting with Redis Sentinel (provides automatic failover) combined with graceful degradation in your application code. This gives you good availability without excessive complexity. As you scale, consider Redis Cluster or geographic replication for even higher resilience.
What's your current setup like - are you on a cloud platform, and what's your scale?