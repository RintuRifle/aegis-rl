# AegisRL — Distributed Rate Limiter & Edge Metering Engine

[![Go](https://img.shields.io/badge/Go-1.23-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Redis](https://img.shields.io/badge/Redis-7-DC382D?logo=redis&logoColor=white)](https://redis.io)
[![Next.js](https://img.shields.io/badge/Next.js-16-000000?logo=nextdotjs&logoColor=white)](https://nextjs.org)
[![Tailwind](https://img.shields.io/badge/Tailwind_CSS-38B2AC?logo=tailwindcss&logoColor=white)](https://tailwindcss.com)
[![Docker](https://img.shields.io/badge/Docker-2496ED?logo=docker&logoColor=white)](https://www.docker.com)

<br/>

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![GitHub Repo](https://img.shields.io/badge/GitHub-Repository-black?logo=github)](https://github.com/RintuRifle/aegis-rl)

A high-performance, production-grade distributed rate limiter built in **Go** with **atomic Redis Lua scripting**, featuring circuit breaker resilience, in-process fallback, Prometheus observability, and a real-time **Next.js dashboard**.

## Architecture

```
                                    ┌──────────────────────────────┐
                                    │     Next.js Dashboard         │
                                    │  (Vercel) — real-time metrics │
                                    └───────────────▲───────────────┘
                                                    │ poll /api/stats
 ┌────────────┐     HTTPS      ┌───────────────────┴────────────────────┐
 │   Client    │───────────────▶│       AEGISRL EDGE BOUNCER (Go)        │
 │ (API caller)│                │  ── Caddy TLS termination + LB ──      │
 └────────────┘                │                                          │
                                │  1. Extract identity (API key > IP)     │
                                │  2. Redis EVALSHA (atomic Lua script)   │
                                │  3. Circuit breaker wraps Redis call    │
                                │  4. Fail-open to local token bucket    │
                                └──────┬───────────────────────┬─────────┘
                                       │                        │
                          ┌────────────▼──────────┐    ┌────────▼─────────┐
                          │   Redis — Atomic Lua   │    │  Local Fallback   │
                          │   Token Bucket Script  │    │  (sync.Map)       │
                          └────────────────────────┘    └──────────────────┘
```

## Key Features

- **Atomic Token Bucket** — Redis Lua script executes GET→REFILL→CHECK→DECREMENT in a single RTT with zero race conditions
- **Circuit Breaker** — Closed→Open→HalfOpen state machine prevents cascading failures when Redis is down
- **Fail-Open Fallback** — In-process `sync.Map`-based token bucket keeps API available in degraded mode
- **Multi-Tier Limits** — Free / Pro / Enterprise tiers with different capacity and refill rates per API key
- **Spoof-Safe Identity** — API key priority with XFF last-hop-only IP extraction
- **Prometheus Metrics** — Request counters, decision latency histograms, circuit breaker gauge
- **Structured Logging** — Production JSON logging via `zap` (not `fmt.Println`)
- **Real-Time Dashboard** — Next.js + Tailwind + Recharts with live traffic charts and request tester

## Quick Start

### Prerequisites
- Go 1.23+
- Docker & Docker Compose
- Node.js 22+ (for dashboard)

### Local Development

```bash
# 1. Start Redis
docker run -d --name redis -p 6379:6379 redis:7-alpine

# 2. Run the Go engine
go run ./cmd/server

# 3. Test it
curl -H "X-API-Key: test123" http://localhost:8081/api/test
curl http://localhost:8081/healthz

# 4. Start dashboard (separate terminal)
cd dashboard && npm install && npm run dev
```

### Docker Compose (Full Stack)

```bash
# Start everything: Redis + 2x Go replicas + Caddy + Prometheus
make docker-up

# View logs
make docker-logs

# Tear down
make docker-down
```

## Project Structure

```
aegis-rl/
├── cmd/server/main.go              # Entrypoint — wires config, Redis, middleware, server
├── internal/
│   ├── limiter/
│   │   ├── limiter.go              # Core Allow() method + Redis EVALSHA
│   │   ├── lua.go                  # go:embed Lua script + SCRIPT LOAD
│   │   ├── local_fallback.go       # sync.Map degraded-mode bucket
│   │   ├── circuit_breaker.go      # Closed/Open/HalfOpen state machine
│   │   └── *_test.go              # Unit tests (20 passing)
│   ├── middleware/
│   │   ├── ratelimit.go            # HTTP middleware — headers, 429, metrics
│   │   ├── identity.go             # API key + spoof-safe IP extraction
│   │   ├── cors.go                 # CORS for dashboard
│   │   └── middleware_test.go
│   ├── config/config.go            # Env-based config + multi-tier
│   ├── metrics/prometheus.go       # Prometheus counters + histograms
│   ├── logging/logger.go           # Structured zap logger
│   └── handlers/handlers.go        # Health, stats, demo, config handlers
├── scripts/token_bucket.lua        # Atomic Redis Lua script
├── dashboard/                      # Next.js + Tailwind real-time dashboard
├── deployments/
│   ├── docker-compose.yml          # Full stack (2x Go + Redis + Caddy + Prometheus)
│   ├── Caddyfile                   # Load balancer + auto-HTTPS
│   └── prometheus.yml              # Scrape config
├── bench/                          # Vegeta load test scripts
├── .github/workflows/ci.yml        # CI: go vet + go test -race + Docker build
├── Makefile                        # build, test, race, bench, docker, pprof
└── Dockerfile                      # Multi-stage: golang:1.23 → alpine
```

## How It Works

### Token Bucket Algorithm (Lua Script)

```lua
tokens(t) = min(capacity, tokens(t_last) + elapsed * refill_rate)
if tokens >= requested then tokens -= requested; allowed = 1 end
```

- **O(1) memory** per client (2 fields: tokens + timestamp)
- **Single atomic Redis operation** — no read-modify-write race window
- **EVALSHA** (not EVAL) — sends only the SHA1 hash, not the full script text
- **TTL auto-eviction** — idle keys expire at `2x` full refill time

### Circuit Breaker Flow

```
Normal: [Closed] → Redis EVALSHA succeeds → stay Closed
Failure: [Closed] → 5 consecutive Redis failures → [Open] → use LocalFallback
Recovery: [Open] → cooldown expires → [HalfOpen] → probe Redis
  → success → [Closed]
  → failure → [Open] (restart cooldown)
```

## API

| Endpoint | Method | Auth | Description |
|---|---|---|---|
| `/healthz` | GET | No | Health check (Caddy uses this) |
| `/api/test` | GET | Optional | Demo endpoint (rate limited) |
| `/api/stats` | GET | No | Real-time stats JSON |
| `/api/config` | GET | No | Current tier configuration |
| `/metrics` | GET | No | Prometheus metrics (port 9100) |

### Response Headers

Every response includes:
- `X-RateLimit-Limit` — max tokens (burst capacity)
- `X-RateLimit-Remaining` — tokens left
- `X-RateLimit-Reset` — Unix timestamp when bucket is full
- `X-RateLimit-Mode` — `degraded` if using local fallback
- `Retry-After` — seconds to wait (on 429 only)

## Testing

```bash
# Run all tests
make test

# Run with race detector (requires CGO)
make race

# Static analysis
make vet

# Escape analysis (memory profiling evidence)
make escape
```

## Benchmarking

```bash
# Vegeta load tests (requires vegeta CLI)
make vegeta

# Go benchmarks
make bench
```

## Configuration

| Env Variable | Default | Description |
|---|---|---|
| `LISTEN_ADDR` | `:8081` | Main server address |
| `METRICS_ADDR` | `:9100` | Prometheus metrics address |
| `REDIS_ADDR` | `localhost:6379` | Redis connection |
| `CAPACITY` | `100` | Default burst size |
| `REFILL_RATE` | `10` | Default tokens/sec |
| `TIMEOUT_MS` | `50` | Redis call timeout |
| `LOG_LEVEL` | `info` | Log level |
| `DASHBOARD_ORIGIN` | `http://localhost:3000` | CORS origin |
| `TIERS` | (defaults) | JSON array of tier configs |

## License

MIT
