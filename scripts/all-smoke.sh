#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
INGEST_API_KEY="${INGEST_API_KEY:-}"
READ_API_KEY="${READ_API_KEY:-}"

HTTP_MARKER="ALL_HTTP_$(date +%s)_$RANDOM"

auth_ingest=()
if [[ -n "$INGEST_API_KEY" ]]; then
  auth_ingest=(-H "Authorization: Bearer ${INGEST_API_KEY}")
fi

auth_read=()
if [[ -n "$READ_API_KEY" ]]; then
  auth_read=(-H "Authorization: Bearer ${READ_API_KEY}")
fi

wait_url() {
  local url=$1
  for _ in $(seq 1 60); do
    if curl -sf "$url" >/dev/null; then
      return 0
    fi
    sleep 2
  done
  echo "timeout waiting for $url" >&2
  return 1
}

wait_url "${BASE_URL}/healthz"

ts="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
curl -sf -X POST "${BASE_URL}/v1/logs" \
  -H "Content-Type: application/json" \
  "${auth_ingest[@]}" \
  -d "{\"logs\":[{\"timestamp\":\"${ts}\",\"level\":\"info\",\"service\":\"all-smoke\",\"message\":\"${HTTP_MARKER}\"}]}"

for _ in $(seq 1 45); do
  if curl -sf "${auth_read[@]}" "${BASE_URL}/v1/logs/search?q=${HTTP_MARKER}&limit=5" | grep -q "$HTTP_MARKER"; then
    echo "search ok"
    break
  fi
  sleep 2
done
curl -sf "${auth_read[@]}" "${BASE_URL}/v1/logs/search?q=${HTTP_MARKER}&limit=5" | grep -q "$HTTP_MARKER"

SSE_FILE=$(mktemp)
curl -sfN --max-time 30 "${auth_read[@]}" "${BASE_URL}/v1/live" >"$SSE_FILE" &
SSE_PID=$!
sleep 2
curl -sf -X POST "${BASE_URL}/v1/logs" \
  -H "Content-Type: application/json" \
  "${auth_ingest[@]}" \
  -d "{\"logs\":[{\"timestamp\":\"${ts}\",\"level\":\"info\",\"service\":\"all-smoke\",\"message\":\"${HTTP_MARKER}_live\"}]}"
for _ in $(seq 1 30); do
  if grep -q "${HTTP_MARKER}_live" "$SSE_FILE" 2>/dev/null; then
    echo "live ok"
    kill "$SSE_PID" 2>/dev/null || true
    exit 0
  fi
  sleep 2
done
echo "SSE did not receive live marker" >&2
kill "$SSE_PID" 2>/dev/null || true
exit 1
