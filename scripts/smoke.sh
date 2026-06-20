#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
TMP=$(mktemp -d)
PORT=${SMOKE_PORT:-18080}
PID=""
cleanup() {
  if [ -n "$PID" ]; then kill "$PID" 2>/dev/null || true; fi
  rm -rf "$TMP"
}
trap cleanup EXIT INT TERM

cp "$ROOT/data/items.sample.json" "$TMP/items.json"
cd "$ROOT"
CGO_ENABLED=0 go build -o "$TMP/signal-garden" ./cmd/server
INITIAL_REFRESH=false DATA_PATH="$TMP/items.json" ADDR="127.0.0.1:$PORT" INBOX_TOKEN=smoke-secret "$TMP/signal-garden" >"$TMP/server.log" 2>&1 &
PID=$!

attempt=0
until curl --fail --silent "http://127.0.0.1:$PORT/healthz" >/dev/null 2>&1; do
  attempt=$((attempt + 1))
  if [ "$attempt" -gt 40 ]; then
    cat "$TMP/server.log"
    exit 1
  fi
  sleep 0.25
done

curl --fail --silent "http://127.0.0.1:$PORT/" | grep -q "Selected work"
STATUS=$(curl --silent --output /dev/null --write-out "%{http_code}" "http://127.0.0.1:$PORT/inbox")
[ "$STATUS" = "303" ]
curl --fail --silent --cookie-jar "$TMP/cookies" -X POST \
  --data-urlencode "token=smoke-secret" \
  "http://127.0.0.1:$PORT/unlock?next=%2Finbox" >/dev/null
curl --fail --silent --cookie "$TMP/cookies" "http://127.0.0.1:$PORT/inbox?q=distributed" | grep -q "Technical signal index"
curl --fail --silent --cookie "$TMP/cookies" "http://127.0.0.1:$PORT/api/search?q=spanner" | grep -q "sample-spanner"
EVENTS=$(curl --silent --max-time 1 "http://127.0.0.1:$PORT/events" || true)
printf '%s' "$EVENTS" | grep -q "viewer_count"
echo "smoke test passed"
