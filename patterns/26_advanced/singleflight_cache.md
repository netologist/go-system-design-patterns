# Singleflight with Stale-While-Revalidate Cache Fallback

## 1. Overview & Concept
This advanced pattern combines two powerful resiliency mechanisms:
1. **Singleflight Deduplication**: Collapses concurrent cache misses for the same key into a single database query, eliminating the Thundering Herd / Cache Stampede effect.
2. **Stale Cache Fallback**: If the database is completely down or returns an error, the service falls back to returning the previous stale cache value (`isStale = true`) to maintain high availability.

## 2. Production Problem & Failure Modes
1. **Cache Stampede Outages**: When a popular cache key expires on a high-traffic endpoint, thousands of simultaneous requests flood the database with identical queries, causing high CPU spikes and cascading connection pool exhaustion.
2. **Complete Availability Outages on Cache Miss**: If the database crashes, ordinary cache implementations fail immediately for any key whose TTL expired, turning a partial database failure into a 100% user-facing outage.

## 3. Architecture & Mechanism

```text
Request 1, 2, ... 50 (Key: "product:123")
               |
               v
       Cache Miss?
               |
               v
       Singleflight Group
               |
         (1 execution)
               |
               +---> DB Success ---> Store in Cache & Stale Backup ---> Return Fresh Data (200)
               |
               +---> DB Fails   ---> Read Stale Backup Record     ---> Return Stale Data (isStale=true)
```

## 4. Production Hardening & Trade-offs
- **Availability over Strictest Freshness**: Returning stale data during an active outage is almost always preferable to returning HTTP 500 error pages.
- **Double-Check Inside Singleflight**: Verify cache inside the singleflight closure to avoid duplicate work if a prior concurrent execution populated it right before entry.
- **Header Signals**: Set response headers (e.g. `X-Cache: STALE`) or log metrics so monitoring systems track database degraded mode.

## 5. Code Walkthrough & Usage
See `singleflight_cache.go` and `singleflight_cache_test.go`:
- `SingleflightWithStaleCache[V]`: Coordinates `MemoryCache[V]`, `SingleflightGroup[V]`, and in-memory stale storage.
- `Fetch(ctx, key, ttl, dbQuery)`: Returns fresh or stale value with `isStale` flag.
