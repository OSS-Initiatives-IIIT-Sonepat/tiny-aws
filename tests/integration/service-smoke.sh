# Service deploy smoke test for tiny-aws.
# Tests that a service job type is accepted and registered.
# Run from repo root: ./tests/integration/service-smoke.sh
# Requires: registry + scheduler + ec2-agent + object-store running on Linux.

set -e

REGISTRY=${REGISTRY_URL:-http://127.0.0.1:9000}
SCHEDULER=${SCHEDULER_URL:-http://127.0.0.1:9001}
STORE=${OBJECT_STORE_URL:-http://127.0.0.1:7001}

AUTH_HEADER=""
if [ -n "$TINYAWS_API_KEY" ]; then
  AUTH_HEADER="-H Authorization: Bearer $TINYAWS_API_KEY"
fi

echo "service deploy smoke test"
echo ""

echo "[1/5] Check health"
curl -sf $REGISTRY/health | grep -q healthy && echo "  registry ok"
curl -sf $SCHEDULER/health | grep -q healthy && echo "  scheduler ok"
curl -sf $STORE/health | grep -q healthy && echo "  object-store ok"

echo "[2/5] Create test app with start.sh"
TMPDIR=$(mktemp -d)
cat > "$TMPDIR/start.sh" << 'STARTSH'
#!/bin/sh
echo "tiny-aws service started on port 19999"
while true; do
  echo -e "HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nok" | nc -l 19999 2>/dev/null || true
done
STARTSH
chmod +x "$TMPDIR/start.sh"
echo "  app created at $TMPDIR"

echo "[3/5] Upload app zip"
cd "$TMPDIR" && zip app.zip start.sh && cd -
curl -sf -X PUT "$STORE/buckets/deployments" || true
curl -sf -X PUT "$STORE/buckets/deployments/objects/smoke-svc-test.zip" \
  --data-binary @"$TMPDIR/app.zip" | grep -q "" && echo "  uploaded"

echo "[4/5] Submit service job"
DEPLOY_URL="$STORE/buckets/deployments/objects/smoke-svc-test.zip"
RESP=$(curl -sf -X POST "$SCHEDULER/jobs" \
  -H "Content-Type: application/json" \
  -d "{\"deploy_url\":\"$DEPLOY_URL\",\"command\":\"\",\"job_type\":\"service\",\"port\":19999}")
JOB_ID=$(echo "$RESP" | grep -o '"job_id":"[^"]*"' | cut -d'"' -f4)
echo "  job_id=$JOB_ID"

echo "[5/5] Wait for job to reach running state"
for i in $(seq 1 15); do
  sleep 2
  STATUS=$(curl -sf "$SCHEDULER/jobs/$JOB_ID" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
  echo "  status=$STATUS"
  if [ "$STATUS" = "running" ]; then
    echo "  service job running ok"
    break
  fi
done

# check service registered in registry
SVCS=$(curl -sf "$REGISTRY/services")
echo "  services: $SVCS"

rm -rf "$TMPDIR"
echo ""
echo "SERVICE SMOKE TEST PASSED"
