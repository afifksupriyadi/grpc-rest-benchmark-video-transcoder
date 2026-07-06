#!/usr/bin/env bash
# Orchestrates Scenario A data collection: runs REST+gRPC requests per payload
# size, then triggers Prometheus collection/table generation for that payload
# before moving to the next one.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SAMPLE_DIR="${SAMPLE_DIR:-$HOME/code/thesis/sample-data}"
RUNS="${RUNS:-30}"
CLIENT_OUTPUT_DIR="$ROOT_DIR/client/output"
CLIENT_BIN="/tmp/scenario_a_client"

PAYLOADS=(
  "10mb:${SAMPLE_DIR}/10MB_1080p_fixed.mp4"
  "50mb:${SAMPLE_DIR}/50MB_1080p_fixed.mp4"
  "100mb:${SAMPLE_DIR}/100MB_1080p_fixed.mp4"
)

usage() {
  echo "Usage: $0 [--runs N] [--payloads 10mb,50mb,100mb]"
  exit 1
}

SELECTED_PAYLOADS=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --runs) RUNS="$2"; shift 2 ;;
    --payloads) SELECTED_PAYLOADS="$2"; shift 2 ;;
    -h|--help) usage ;;
    *) echo "Unknown argument: $1"; usage ;;
  esac
done

echo "== Checking Docker stack =="
if ! curl -sf "http://localhost:9090/api/v1/targets" >/dev/null; then
  echo "Prometheus is not reachable at localhost:9090. Run 'docker compose up -d' first." >&2
  exit 1
fi
UNHEALTHY=$(curl -s "http://localhost:9090/api/v1/targets" | python3 -c "
import json, sys
data = json.load(sys.stdin)
bad = [t['scrapePool'] for t in data['data']['activeTargets'] if t['health'] != 'up']
print(','.join(bad))
")
if [[ -n "$UNHEALTHY" ]]; then
  echo "Prometheus targets not healthy: $UNHEALTHY" >&2
  exit 1
fi
echo "Prometheus targets OK."

echo "== Building client binary =="
(cd "$ROOT_DIR/client" && go build -o "$CLIENT_BIN" .)

run_requests() {
  local protocol="$1" payload_label="$2" input_file="$3"
  for i in $(seq 1 "$RUNS"); do
    echo "  [$payload_label][$protocol] request $i/$RUNS"
    "$CLIENT_BIN" transcode \
      --protocol "$protocol" \
      --input "$input_file" \
      --scenario scenario_a \
      --output "$CLIENT_OUTPUT_DIR"
  done
}

collect_payload() {
  local payload_label="$1"
  echo "== Collecting Prometheus data for payload $payload_label =="
  sleep 3 # let the last scrape (1s interval) catch up
  python3 "$ROOT_DIR/scripts/collect_scenario_a.py" --payload-size "$payload_label"
}

for entry in "${PAYLOADS[@]}"; do
  label="${entry%%:*}"
  file="${entry#*:}"

  if [[ -n "$SELECTED_PAYLOADS" && ",$SELECTED_PAYLOADS," != *",$label,"* ]]; then
    continue
  fi

  if [[ ! -f "$file" ]]; then
    echo "Sample file not found for $label: $file" >&2
    exit 1
  fi

  echo "=========================================="
  echo "Payload $label ($RUNS runs REST + $RUNS runs gRPC)"
  echo "=========================================="
  run_requests rest "$label" "$file"
  run_requests grpc "$label" "$file"
  collect_payload "$label"
done

echo "Done. Results in $ROOT_DIR/results/scenario_a/"
