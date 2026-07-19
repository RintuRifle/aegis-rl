.PHONY: build test race vet bench chaos docker-up docker-down clean

# ── Build ──────────────────────────────────────────────────
build:
	go build -o bin/aegisrl ./cmd/server

# ── Test ───────────────────────────────────────────────────
test:
	go test -v -count=1 ./...

# ── Race detector (exercises OS concurrency fundamentals) ──
race:
	go test -race -v -count=1 ./...

# ── Static analysis ────────────────────────────────────────
vet:
	go vet ./...

# ── Escape analysis (memory-hierarchy evidence for README) ─
escape:
	go build -gcflags="-m" ./cmd/server 2>&1 | head -60

# ── Benchmarks ─────────────────────────────────────────────
bench:
	go test -bench=. -benchmem ./internal/limiter/...

# ── Vegeta load test ───────────────────────────────────────
vegeta:
	bash bench/run-benchmark.sh

# ── Chaos test: kill Redis mid-load, graph the recovery ────
chaos:
	bash bench/chaos-test.sh

# ── Docker ─────────────────────────────────────────────────
docker-up:
	docker compose -f deployments/docker-compose.yml up --build -d

docker-down:
	docker compose -f deployments/docker-compose.yml down

docker-logs:
	docker compose -f deployments/docker-compose.yml logs -f

# ── pprof (run while server is under load) ─────────────────
pprof-cpu:
	go tool pprof http://localhost:9100/debug/pprof/profile?seconds=30

pprof-heap:
	go tool pprof http://localhost:9100/debug/pprof/heap

# ── Clean ──────────────────────────────────────────────────
clean:
	rm -rf bin/ bench/results/
