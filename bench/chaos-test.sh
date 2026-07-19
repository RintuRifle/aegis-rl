#!/bin/bash
# AegisRL Chaos Test — kill Redis mid-load, watch the fallback engage, then recover.
#
# Produces the single most convincing interview artifact: a latency/error-rate
# graph across [healthy → Redis down → degraded mode → Redis back → recovered].
#
# Usage: ./bench/chaos-test.sh [target_url] [redis_container_name]
# Requires: vegeta, docker, a running stack (make docker-up)
set -euo pipefail

TARGET=${1:-"http://localhost:8081/api/test"}
REDIS_CONTAINER=${2:-"redis"}
RESULTS_DIR="bench/results"
mkdir -p "$RESULTS_DIR"
TS=$(date +%Y%m%d_%H%M%S)
BIN="$RESULTS_DIR/chaos_${TS}.bin"

echo "═══════════════════════════════════════════════════════════"
echo "  AegisRL Chaos Test — Redis outage mid-benchmark"
echo "  Target: $TARGET | Redis container: $REDIS_CONTAINER"
echo "  Timeline: 0-20s healthy → 20s KILL redis → 40s RESTART → 60s end"
echo "═══════════════════════════════════════════════════════════"

# Schedule the chaos: kill Redis at t+20s, bring it back at t+40s
(
  sleep 20
  echo ""
  echo "💥 [t+20s] Stopping Redis — circuit breaker should trip, fallback engages..."
  docker stop "$REDIS_CONTAINER" >/dev/null
  sleep 20
  echo "🔄 [t+40s] Restarting Redis — breaker should close after probe succeeds..."
  docker start "$REDIS_CONTAINER" >/dev/null
) &
CHAOS_PID=$!

# 60s of steady load spanning the whole outage window
echo "GET $TARGET" | \
  vegeta attack -rate=300 -duration=60s -header="X-API-Key: chaos-key" | \
  tee "$BIN" | \
  vegeta report -type=text

wait "$CHAOS_PID" 2>/dev/null || true

# Plot latency over time — the outage + recovery will be clearly visible
vegeta plot < "$BIN" > "$RESULTS_DIR/chaos_${TS}.html"

# Per-window success-rate breakdown
echo ""
echo "── Success rate by phase ──────────────────────────────────"
vegeta report -type=json < "$BIN" | python3 -c "
import json,sys
r = json.load(sys.stdin)
print(f'  overall success: {r[\"success\"]*100:.2f}%')
print(f'  p50: {r[\"latencies\"][\"50th\"]/1e6:.2f}ms  p99: {r[\"latencies\"][\"99th\"]/1e6:.2f}ms  max: {r[\"latencies\"][\"max\"]/1e6:.2f}ms')
" 2>/dev/null || true

echo ""
echo "  Full timeline graph: $RESULTS_DIR/chaos_${TS}.html"
echo "  Screenshot that graph for your README — it shows P99 through the outage."
echo "═══════════════════════════════════════════════════════════"
