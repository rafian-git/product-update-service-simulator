# Value First – Golang Developer Homework


---
## Setup Instructions

After cloning the repository, you can run the service using:

```bash
go run ./cmd/api
```
Or specify environment variables as needed:

```bash
PORT=8080 WORKERS=5 QUEUE_SIZE=128 go run ./cmd/api/
```

## Design Choices

- **Layered Structure:** Clear separation between queue, store, and HTTP layers for modularity.
- **Worker Pool:** Configurable goroutines process events asynchronously for better throughput.
- **Thread-Safe Store:** Uses `sync.RWMutex` to handle concurrent reads/writes safely.
- **Buffered Queue:** Channel-based queue (`QUEUE_SIZE`) manages backpressure effectively.
- **Graceful Shutdown:** Waits until the queue is drained (up to 10 s) before terminating, ensuring no event loss.
- **Testing:** API and race-detection tests ensure correctness and concurrency safety.

## Production Considerations

### Messaging
- **RabbitMQ (Queue):**
    - To meet the given requirement (multiple workers but storing the latest state)- 
      - Route events to one of N queues by hashing a stable key (e.g., product_id)
      - **`prefetchCount`=1** will ensure messages are distributed fairly among workers. 
      - **Acknowledgment** ensures successful processing of messages.
      - **Retry** mechanism for transient errors.
      - **Dead-letter queues** (DLQ) for non-transient errors.
      - **idempotency keys** can be used (e.g., `{product_id, version}` or event UUID) to make replays safe.

### Persistence
- **PostgreSQL (Source of Truth):**
    - Table: `products(product_id PK, price NUMERIC(12,2), stock INT, updated_at TIMESTAMPTZ, version BIGINT)`.
    - **Upserts** (`INSERT ... ON CONFLICT ... DO UPDATE`) with `updated_at` guards for last-write-wins.
    - Create **indexes** on `product_id`, `(updated_at)`.
    - Consider an **Outbox table** for reliable event publishing (Outbox Pattern) with a background relay to RabbitMQ.

### Caching
- **Redis (Cache/Speed Layer):**
    - Store hot product snapshots with TTL; **write-through** on updates, **read-through** on GET.
    - Use **Redis Streams** for light queuing or fan-out (optional), but RabbitMQ remains the primary broker.
    - Eviction policy: `volatile-lru` and explicit cache-busting on update.

### Scale & Throughput
- **Sharded Scaling:** Distribute events across multiple keyed shards or queues so each worker processes a distinct subset serially while enabling parallel throughput.
- **Backpressure:** Tune RabbitMQ prefetch, HTTP rate limiting, and internal queue sizes.
- **Batching:** Consume and apply updates in small batches (e.g., 100–500) to reduce DB round-trips; use **COPY**/bulk upserts for backfills.
- **Observability:** Export **Prometheus** metrics (queue depth, consumer lag, DB latency, error rates), **OpenTelemetry** traces for end-to-end visibility, and structured logs with correlation IDs.

### Reliability, Errors & Retries
- **At-least-once processing:** Enable consumer **ack** after successful apply; on failure, **nack/requeue** with bounded retries.
- **Retry strategy:** Exponential backoff with **retry headers**; after N attempts, route to **DLQ** for manual/automated remediation.
- **Idempotency:** Persist processed event IDs (Redis set with TTL ) to avoid double-apply.
- **Poison messages:** Detect non-transient errors (validation, schema) and send to DLQ immediately; alert on DLQ growth.
- **Readiness/Liveness:** Health endpoints that check DB, broker, and Redis connectivity; **graceful shutdown** that drains in-flight messages before exit.

## Troubleshooting Strategies

### Data Consistency

- **Check metrics & logs:** Verify queue depth, worker errors, DB `rows_affected`, and `apply_errors`.
- **RabbitMQ:** If queue is full or DLQ growing → consumers slow, NACKs, or schema/validation issues.
- **DB:** Upserts may be skipped by LWW(Last-Write-Wins) guards (`updated_at` older) or idempotency checks.
- **Ordering:** Confirm all updates for a product go to the same shard/queue (`prefetch=1`).
- **Observability:** Trace a `product_id` across API → queue → DB.

### When Products Aren’t Updating

1. **API:** Ensure `/events` returns `202` and validation isn’t rejecting data.
2. **Queue:** Confirm messages are enqueued and consumed (no buildup in RabbitMQ).
3. **Workers:** Check logs for `apply_errors`, panics, or context cancellation.
4. **Database:** Inspect `updated_at`; older timestamps may be ignored by LWW logic.
5. **DLQ:** Look for failed events; replay after fix.
6. **Quick fix:** Send a fresh event with a newer timestamp and watch it propagate end-to-end.

