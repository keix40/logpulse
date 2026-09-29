#!/usr/bin/env bash
set -euo pipefail

INGEST_URL="${INGEST_URL:-http://localhost:8080}"
WORKER_URL="${WORKER_URL:-http://localhost:8081}"
MOCK_URL="${MOCK_URL:-http://localhost:9090}"
SYSLOG_HOST="${SYSLOG_HOST:-127.0.0.1}"
SYSLOG_PORT="${SYSLOG_PORT:-5514}"

HTTP_MARKER="E2E_HTTP_$(date +%s)_$RANDOM"
SYSLOG_MARKER="E2E_SYSLOG_$(date +%s)_$RANDOM"
ALERT_MARKER="E2E_SMOKE_ALERT_${RANDOM}"

wait_url() {
  local url=$1
  local name=$2
  for _ in $(seq 1 90); do
    if curl -sf "$url" >/dev/null; then
      echo "$name is up"
      return 0
    fi
    sleep 2
  done
  echo "timeout waiting for $name ($url)" >&2
  return 1
}

wait_url "${INGEST_URL}/healthz" "ingest"
wait_url "${WORKER_URL}/healthz" "worker"
wait_url "${MOCK_URL}/healthz" "mockwebhook"

curl -sf -X POST "${MOCK_URL}/v1/reset" >/dev/null || true

SSE_FILE=$(mktemp)
curl -sfN --max-time 120 "${WORKER_URL}/v1/live" >"$SSE_FILE" &
SSE_PID=$!
sleep 1

ts="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
curl -sf -X POST "${INGEST_URL}/v1/logs" \
  -H "Content-Type: application/json" \
  -d "{\"logs\":[{\"timestamp\":\"${ts}\",\"level\":\"info\",\"service\":\"e2e-http\",\"message\":\"${HTTP_MARKER}\"}]}"

printf '<134>1 %s host e2e-syslog - - - %s\n' "$(date -u +"%Y-%m-%dT%H:%M:%S.000Z")" "$SYSLOG_MARKER" \
  | nc -w 3 "$SYSLOG_ADDR" 5514 || true

curl -sf -X POST "${INGEST_URL}/v1/logs" \
  -H "Content-Type: application/json" \
  -d "{\"logs\":[{\"timestamp\":\"${ts}\",\"level\":\"error\",\"service\":\"e2e-alert\",\"message\":\"${ALERT_MARKER} E2E_SMOKE_ALERT\"}]}"

search_contains() {
  local marker=$1
  for _ in $(seq 1 45); do
    if curl -sf "${WORKER_URL}/v1/logs/search?q=${marker}&limit=20" | grep -q "$marker"; then
      echo "search saw ${marker}"
      return 0
    fi
    sleep 2
  done
  echo "search never saw ${marker}" >&2
  return 1
}

search_contains "$HTTP_MARKER"
search_contains "$SYSLOG_MARKER"

for _ in $(seq 1 45); do
  if grep -q "$HTTP_MARKER" "$SSE_FILE" 2>/dev/null; then
    echo "SSE saw HTTP marker"
    break
  fi
  sleep 2
done
grep -q "$HTTP_MARKER" "$SSE_FILE" || { echo "SSE missing HTTP marker" >&2; kill $SSE_PID 2>/dev/null || true; exit 1; }

kill $SSE_PID 2>/dev/null || true

for _ in $(seq 1 45); do
  if curl -sf "${MOCK_URL}/v1/requests" | grep -q "E2E_SMOKE_ALERT"; then
    echo "mock webhook received alert"
    exit 0
  fi
  sleep 2
done

echo "alert webhook was not called" >&2
curl -sf "${MOCK_URL}/v1/requests" || true
exit 1
