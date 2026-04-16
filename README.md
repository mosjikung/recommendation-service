# Recommendation Service

A production-ready backend recommendation service built with Go, PostgreSQL, Redis, and Docker.

## Quick Start

```bash
docker-compose up --build
```

The API will be available at `http://localhost:8080`.

---

## Setup Instructions

### Prerequisites

- Docker & Docker Compose
- Go 1.22+ (for local development only)
- k6 (for performance testing)

### Run with Docker (recommended)

```bash
# 1. Start all services (app + postgres + redis)
docker-compose up --build

# 2. Seed the database (in a separate terminal)
docker-compose exec app go run scripts/seed.go
```

### Run locally

```bash
# Start dependencies
docker-compose up postgres redis

# Run migrations (auto-applied via docker-entrypoint-initdb.d on postgres)
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

**Response:**
```json
{
  "user_id": 1,
  "recommendations": [
    {
      "content_id": 42,
      "title": "Action Title 5",
      "genre": "action",
      "popularity_score": 0.87,
      "score": 0.74
    }
  ],
  "metadata": {
    "cache_hit": false,
    "generated_at": "2026-02-15T10:30:00Z",
    "total_count": 10
  }
}
```

### Batch Recommendations

```
GET /recommendations/batch?page=1&limit=20
```

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
TTL of 10 minutes balances freshness against DB load. Keys are scoped by `user_id` and `limit` so a user requesting 5 vs 10 recommendations gets independent cache entries. Cache is invalidated on watch history updates.

### Concurrency Control
The batch endpoint uses a semaphore-based worker pool capped at 10 concurrent goroutines. This prevents connection pool exhaustion under high batch load while keeping latency low.

### Error Handling
Service errors are typed (sentinel errors) so handlers can map them precisely to HTTP status codes without string matching.

### Scoring Algorithm Weights
| Component   | Weight | Rationale |
|-------------|--------|-----------|
| Popularity  | 0.40   | Dominant signal — popular content converts well |
| Genre match | 0.35   | Personalisation signal from watch history |
| Recency     | 0.15   | Slight freshness bias to surface new content |
| Noise       | 0.10   | Exploration — prevents filter bubbles |

---

## Performance Testing

```bash
# Install k6
brew install k6   # macOS
# or: https://k6.io/docs/getting-started/installation/

# Run all scenarios
k6 run tests/k6/load_test.js
```

### Test Scenarios

1. **Single User Load Test** — 100 RPS for 1 minute against `/users/{id}/recommendations`
2. **Batch Stress Test** — Ramping VUs against `/recommendations/batch` with varying page sizes
3. **Cache Effectiveness Test** — Repeated requests to the same 3 users to measure cache hit ratio

### Thresholds

| Metric | Threshold |
|--------|-----------|
| p95 latency | < 500ms |
| p99 latency | < 1000ms |
| Error rate | < 5% (includes simulated 1.5% model failures) |

---

## Trade-offs & Future Improvements

**Known limitations:**
- Model client is synchronous; a real ML model would call an external gRPC endpoint
- No authentication / API key validation
- Watch history cache invalidation uses SCAN which can be slow on large Redis keyspaces

**Proposed enhancements:**
- Add circuit breaker around model client calls
- Pre-warm cache for top-1000 active users during off-peak hours
- Use Redis Pipeline for batch cache operations
- Add Prometheus metrics endpoint for observability
