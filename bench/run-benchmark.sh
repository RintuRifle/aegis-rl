#!/bin/bash
# AegisRL Benchmark Suite — Vegeta Load Testing
# Usage: ./bench/run-benchmark.sh [target_url]
set -euo pipefail

TARGET=${1:-"http://localhost:8081/api/test"}
RESULTS_DIR="bench/results"
mkdir -p "$RESULTS_DIR"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

echo "═══════════════════════════════════════════════════════════"
echo "  AegisRL Benchmark Suite"
echo "  Target: $TARGET"
echo "  Time: $TIMESTAMP"
echo "═══════════════════════════════════════════════════════════"

# ── Test 1: Steady State — 500 RPS for 30s ────────────────
echo ""
echo "▶ Test 1: Steady 500 RPS for 30s"
echo "GET $TARGET" | \
  vegeta attack -rate=500 -duration=30s -header="X-API-Key: bench-key-001" | \
  tee "$RESULTS_DIR/steady_${TIMESTAMP}.bin" | \
  vegeta report -type=text > "$RESULTS_DIR/steady_${TIMESTAMP}.txt"
cat "$RESULTS_DIR/steady_${TIMESTAMP}.txt"

# ── Test 2: Burst — 2000 RPS for 5s ──────────────────────
echo ""
echo "▶ Test 2: Burst 2000 RPS for 5s"
echo "GET $TARGET" | \
  vegeta attack -rate=2000 -duration=5s -header="X-API-Key: bench-key-002" | \
  tee "$RESULTS_DIR/burst_${TIMESTAMP}.bin" | \
  vegeta report -type=text > "$RESULTS_DIR/burst_${TIMESTAMP}.txt"
cat "$RESULTS_DIR/burst_${TIMESTAMP}.txt"

# ── Test 3: Ramp — 100 to 1000 RPS ───────────────────────
echo ""
echo "▶ Test 3: Ramp 100→1000 RPS over 30s"
for rate in 100 250 500 750 1000; do
  echo "  → Testing at ${rate} RPS for 5s..."
  echo "GET $TARGET" | \
    vegeta attack -rate=$rate -duration=5s -header="X-API-Key: bench-key-003" | \
    vegeta report -type=text | grep -E "Latencies|Success|Status"
done

# ── Generate HTML Report ─────────────────────────────────
echo ""
echo "▶ Generating HTML latency plot..."
cat "$RESULTS_DIR/steady_${TIMESTAMP}.bin" | \
  vegeta plot > "$RESULTS_DIR/report_${TIMESTAMP}.html"

echo ""
echo "═══════════════════════════════════════════════════════════"
echo "  Results saved to $RESULTS_DIR/"
echo "  HTML report: $RESULTS_DIR/report_${TIMESTAMP}.html"
echo "═══════════════════════════════════════════════════════════"
