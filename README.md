# Recommendation Service

Production-ready backend recommendation service built with Go, PostgreSQL, Redis, and Docker. Implements a layered architecture with Redis caching, concurrent batch processing, and a heuristic-based scoring algorithm simulating ML inference.

## Quick Start

```bash
# 1. Start all services
docker-compose up --build

# 2. Seed the database (separate terminal)
docker-compose --profile tools run --rm seeder

# 3. Run performance tests (optional)
docker-compose --profile tools run --rm k6
```

The API will be available at `http://localhost:8080`.

---

## Setup Instructions

### Prerequisites

- Docker & Docker Compose

### Run with Docker (recommended)

```bash
# Start all services (app + postgres + redis)
docker-compose up --build

# Seed database
docker-compose --profile tools run --rm seeder
```

### Run locally

```bash
# Start dependencies
docker-compose up postgres redis

# Run migrations
psql $DATABASE_URL -f migrations/001_create_tables.sql

# Seed
DATABASE_URL=postgres://user:password@localhost:5432/recommendations?sslmode=disable \
  go run scripts/seed.go

# Start server
go run ./cmd/server
```

---

## API Endpoints

### Single User Recommendations

```
GET /users/{user_id}/recommendations?limit=10
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| user_id | integer | yes | Unique identifier of the user |
| limit | integer | no | Number of recommendations (default: 10, max: 50) |

**Response:**
```json
{
  "user_id": 1,
  "recommendations": [
    {
      "content_id": 8,
      "title": "Action Title 8",
      "genre": "action",
      "popularity_score": 0.87,
      "score": 0.53
    }
  ],
  "metadata": {
    "cache_hit": false,
    "generated_at": "2026-04-12T14:53:15Z",
    "total_count": 10
  }
}
```

### Batch Recommendations

```
GET /recommendations/batch?page=1&limit=20
```

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| page | integer | no | Page number (default: 1, min: 1) |
| limit | integer | no | Users per page (default: 20, max: 100) |

### Error Responses

| Status | Error Code | Description |
|--------|-----------|-------------|
| 400 | invalid_parameter | Invalid query parameter |
| 404 | user_not_found | User does not exist |
| 500 | internal_error | Unexpected server error |
| 503 | model_unavailable | Model inference failed |

---

## Architecture Overview

```
┌─────────────────────────────────┐
│         Handler Layer           │  HTTP routing, input validation,
│  (internal/handler/handler.go)  │  response serialization
└────────────────┬────────────────┘
                 │
┌────────────────▼────────────────┐
│         Service Layer           │  Business logic, cache orchestration,
│  (internal/service/service.go)  │  batch concurrency
└──────┬──────────────────┬───────┘
       │                  │
┌──────▼──────┐    ┌──────▼──────┐
│  Repository │    │Model Client │  Repository: SQL queries
│   Layer     │    │             │  Model: heuristic scoring algorithm
└──────┬──────┘    └─────────────┘
       │
┌──────▼────────────────────┐
│  PostgreSQL  +  Redis      │  Persistent storage + recommendation cache
└────────────────────────────┘
```

### Layer Responsibilities

**Handler** — Parses HTTP requests, validates parameters, maps service errors to HTTP status codes.

**Service** — Orchestrates the recommendation pipeline: cache check → DB fetch → model scoring → cache store. Manages bounded concurrency for batch processing using a worker pool (10 goroutines).

**Repository** — All SQL queries live here. No business logic. Avoids N+1 by fetching watch history and candidates in single queries.

**Model Client** — Simulates ML inference with a heuristic scoring function. Introduces 30–50ms latency and 1.5% random failure rate to mimic production ML behaviour.

**Cache** — Redis-backed with structured keys (`rec:user:{id}:limit:{n}`) and 10-minute TTL.

---

## Design Decisions

### Caching Strategy
TTL of 10 minutes balances freshness against DB load. Keys are scoped by `user_id` and `limit` so a user requesting 5 vs 10 recommendations gets independent cache entries. Cache is invalidated on watch history updates via Redis SCAN + DEL pattern.

### Concurrency Control
The batch endpoint uses a semaphore-based worker pool capped at 10 concurrent goroutines. This prevents connection pool exhaustion under high batch load while keeping latency low.

### Error Handling
Service errors are typed (sentinel errors) so handlers can map them precisely to HTTP status codes without string matching.

### Scoring Algorithm Weights
| Component | Weight | Rationale |
|-----------|--------|-----------|
| Popularity | 0.40 | Dominant signal — popular content converts well |
| Genre match | 0.35 | Personalisation signal from watch history |
| Recency | 0.15 | Slight freshness bias to surface new content |
| Noise | 0.10 | Exploration — prevents filter bubbles |

### Database Indexing
| Index | Purpose |
|-------|---------|
| idx_watch_history_user | Speeds up fetching watch history for a specific user |
| idx_watch_history_composite | Optimizes queries that need recent watch history ordered by time |
| idx_content_genre | Enables fast filtering by genre for recommendation candidates |
| idx_content_popularity | DESC ordering helps quickly fetch top popular content |

---

## Performance Results

Test environment: Docker on Windows 10, 3 scenarios running sequentially.

### k6 Test Results

```
scenarios: 3 scenarios, 209 max VUs, 3m30s total duration
  - single_user_load:    100 RPS for 1 minute
  - batch_stress:        up to 30 VUs for 1m20s
  - cache_effectiveness: 5 VUs for 30s
```

### Metrics Summary

| Metric | Result | Threshold | Status |
|--------|--------|-----------|--------|
| avg latency | 0.65ms | - | ✅ |
| p(90) latency | 1.08ms | - | ✅ |
| p(95) latency | 1.35ms | < 500ms | ✅ |
| p(99) latency | 1.87ms | < 1000ms | ✅ |
| max latency | 7.08ms | - | ✅ |
| error rate | 0.00% | < 5% | ✅ |
| throughput | 41.65 req/s | - | ✅ |
| total requests | 7,501 | - | - |
| cache hit rate | 100% | - | ✅ |

### Cache Hit Rate Analysis
Cache hit rate reached 100% during the `cache_effectiveness` scenario where the same 3 users were requested repeatedly. This confirms Redis caching is functioning correctly — subsequent requests within the 10-minute TTL window are served entirely from cache without touching PostgreSQL.

### Bottlenecks and Limiting Factors
The simulated 30–50ms model latency is the dominant factor on cache-miss requests. On cache-hit requests, latency drops to sub-millisecond range (avg 0.65ms), demonstrating that Redis effectively eliminates the DB + model overhead for repeated requests.

The `has results` check failed for 2.52% of batch requests, which is expected behaviour — the 1.5% simulated model failure rate causes some per-user results to return as failed status within the batch response.

---

## Trade-offs & Future Improvements

**Known limitations:**
- Model client is synchronous; a real ML model would call an external gRPC endpoint
- No authentication / API key validation
- Watch history cache invalidation uses SCAN which can be slow on large Redis keyspaces

**Proposed enhancements:**
- Add interface abstraction on Repository and Cache layers for easier testing and swapping implementations
- Add circuit breaker around model client calls
- Pre-warm cache for top active users during off-peak hours
- Use Redis Pipeline for batch cache operations
- Add Prometheus metrics endpoint for observability
- Move hardcoded values (worker pool size, TTL, DB pool size) to environment config
