#!/bin/bash
# Service deploy smoke test for tiny-aws on Linux.
# Tests that a service job type is accepted, spawned, and registered.
# Requires: registry + scheduler + ec2-agent + object-store running.
# Run from repo root: ./tests/integration/service-smoke.sh

set -e

REGISTRY=${REGISTRY_URL:-http://127.0.0.1:9000}
SCHEDULER=${SCHEDULER_URL:-http://127.0.0.1:9001}
STORE=${OBJECT_STORE_URL:-http://127.0.0.1:7001}

CURL_AUTH=()
if [ -n "$TINYAWS_API_KEY" ]; then
  CURL_AUTH=(-H "Authorization: Bearer $TINYAWS_API_KEY")
fi

ok()   { echo "  $1 ok"; }
fail() { echo "FAIL: $1"; exit 1; }

echo "service deploy smoke test"
echo ""

echo "[1/6] Check health"
curl -sf "${CURL_AUTH[@]}" "$REGISTRY/health" | grep -q healthy || fail "registry"
ok "registry"
curl -sf "$STORE/health" | grep -q healthy || fail "object-store"
ok "object-store"
curl -sf "${CURL_AUTH[@]}" "$SCHEDULER/health" | grep -q healthy || fail "scheduler"
ok "scheduler"

echo "[2/6] Create test app with start.sh"
WORK_DIR=$(mktemp -d)            # avoid clobbering $TMPDIR
trap 'rm -rf "$WORK_DIR"' EXIT

cat > "$WORK_DIR/start.sh" << 'STARTSH'
#!/bin/sh
echo "tiny-aws service started"
while true; do
  printf "HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nok" | nc -l -p 19999 2>/dev/null || true
done
STARTSH
chmod +x "$WORK_DIR/start.sh"
ok "app created at $WORK_DIR"

echo "[3/6] Upload app zip"
(cd "$WORK_DIR" && zip app.zip start.sh > /dev/null)
curl -sf "${CURL_AUTH[@]}" -X PUT "$STORE/buckets/deployments" > /dev/null 2>&1 || true
curl -sf "${CURL_AUTH[@]}" -X PUT "$STORE/buckets/deployments/objects/smoke-svc-test.zip" \
  --data-binary @"$WORK_DIR/app.zip" > /dev/null || fail "upload zip"
ok "uploaded"

echo "[4/6] Submit service job"
DEPLOY_URL="$STORE/buckets/deployments/objects/smoke-svc-test.zip"
RESP=$(curl -sf "${CURL_AUTH[@]}" -X POST "$SCHEDULER/jobs" \
  -H "Content-Type: application/json" \
  -d "{\"deploy_url\":\"$DEPLOY_URL\",\"command\":\"\",\"job_type\":\"service\",\"port\":19999}")
JOB_ID=$(echo "$RESP" | grep -o '"job_id":"[^"]*"' | cut -d'"' -f4)
[ -n "$JOB_ID" ] || fail "job_id missing from response: $RESP"
ok "job $JOB_ID submitted"

echo "[5/6] Wait for job to reach running state"
STATUS=""
for i in $(seq 1 15); do
  sleep 2
  STATUS=$(curl -sf "${CURL_AUTH[@]}" "$SCHEDULER/jobs/$JOB_ID" \
    | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
  echo "  status=$STATUS"
  [ "$STATUS" = "running" ] && break
  [ "$STATUS" = "failed" ] && {
    DETAIL=$(curl -sf "${CURL_AUTH[@]}" "$SCHEDULER/jobs/$JOB_ID")
    fail "job failed: $DETAIL"
  }
done
[ "$STATUS" = "running" ] || fail "job never reached running (last status=$STATUS)"
ok "service job running"

echo "[6/6] Verify service registered in registry"
# allow up to 15s for agent to register the service
SVC_ID=""
for i in $(seq 1 5); do
  sleep 3
  SVC_ID=$(curl -sf "${CURL_AUTH[@]}" "$REGISTRY/services" \
    | grep -o '"id":"svc-[^"]*"' | head -1 | cut -d'"' -f4)
  [ -n "$SVC_ID" ] && break
done
[ -n "$SVC_ID" ] || fail "service not registered in registry after 15s"
ok "service $SVC_ID registered"

echo ""
echo "SERVICE SMOKE TEST PASSED"
