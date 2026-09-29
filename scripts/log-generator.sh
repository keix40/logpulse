#!/usr/bin/env bash
set -euo pipefail

INGEST_URL="${INGEST_URL:-http://localhost:8080/v1/logs}"
INTERVAL="${INTERVAL:-0.5}"

services=(api worker payments auth)
levels=(info info info warn error debug)

echo "Sending logs to ${INGEST_URL} (Ctrl+C to stop)"

while true; do
  svc="${services[RANDOM % ${#services[@]}]}"
  lvl="${levels[RANDOM % ${#levels[@]}]}"
  msg="sample event from ${svc} seq=$((RANDOM))"
  if [[ "$lvl" == "error" ]]; then
    msg="request failed: upstream timeout service=${svc}"
  fi
  ts="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
  payload=$(cat <<EOF
{"logs":[{"timestamp":"${ts}","level":"${lvl}","service":"${svc}","message":"${msg}"}]}
EOF
)
  curl -sf -X POST "$INGEST_URL" \
    -H "Content-Type: application/json" \
    -d "$payload" >/dev/null || echo "ingest request failed" >&2
  sleep "$INTERVAL"
done
