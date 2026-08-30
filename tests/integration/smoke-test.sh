#!/bin/bash
# Full-stack integration smoke test for tiny-aws on Linux.
# Assumes the stack is running (use ./scripts/run-local.sh first).
# Run from repo root: ./tests/integration/smoke-test.sh

set -e

REGISTRY=${REGISTRY_URL:-http://127.0.0.1:9000}
SCHEDULER=${SCHEDULER_URL:-http://127.0.0.1:9001}
STORE=${OBJECT_STORE_URL:-http://127.0.0.1:7001}

# optional bearer auth
CURL_AUTH=()
IRM_AUTH=()
if [ -n "$TINYAWS_API_KEY" ]; then
  CURL_AUTH=(-H "Authorization: Bearer $TINYAWS_API_KEY")
  IRM_AUTH=(-H "Authorization: Bearer $TINYAWS_API_KEY")
fi

ok() { echo "  $1 ok"; }
fail() { echo "FAIL: $1"; exit 1; }

echo "tiny-aws integration smoke test (Linux)"
echo ""

echo "[1/9] Service health"
curl -sf "${CURL_AUTH[@]}" "$REGISTRY/health" | grep -q healthy || fail "registry health"
ok "registry"
curl -sf "$STORE/health" | grep -q healthy || fail "object-store health"
ok "object-store"
curl -sf "${CURL_AUTH[@]}" "$SCHEDULER/health" | grep -q healthy || fail "scheduler health"
ok "scheduler"

echo ""
echo "[2/9] Registry nodes"
curl -sf "${CURL_AUTH[@]}" "$REGISTRY/nodes" | grep -q "{" || fail "nodes list"
ok "nodes"
curl -sf "${CURL_AUTH[@]}" "$REGISTRY/nodes?role=compute" | grep -q "{" || fail "compute nodes"
ok "compute nodes"

echo ""
echo "[3/9] Object store PUT/GET/meta"
KEY="smoke-$(date +%s)"
curl -sf "${CURL_AUTH[@]}" -X PUT "$STORE/objects/$KEY" -d "hello integration" || fail "object PUT"
ok "PUT"
BODY=$(curl -sf "${CURL_AUTH[@]}" "$STORE/objects/$KEY")
[ "$BODY" = "hello integration" ] || fail "object GET mismatch: $BODY"
ok "GET"
curl -sf "${CURL_AUTH[@]}" "$STORE/objects/$KEY/meta" | grep -q "$KEY" || fail "object meta"
ok "meta"

echo ""
echo "[4/9] Scheduler"
curl -sf "${CURL_AUTH[@]}" "$SCHEDULER/schedule" | grep -q node_id || fail "schedule"
ok "schedule"

echo ""
echo "[5/9] Job submission"
JOB=$(curl -sf "${CURL_AUTH[@]}" -X POST "$SCHEDULER/jobs" \
  -H "Content-Type: application/json" -d '{"command":"echo hello"}')
JOB_ID=$(echo "$JOB" | grep -o '"job_id":"[^"]*"' | cut -d'"' -f4)
NODE_ID=$(echo "$JOB" | grep -o '"node_id":"[^"]*"' | cut -d'"' -f4)
[ -n "$JOB_ID" ] || fail "job_id missing"
[ -n "$NODE_ID" ] || fail "node_id missing"
ok "submit job $JOB_ID"

echo ""
echo "[6/9] Job execution"
FINAL=""
for i in $(seq 1 15); do
  sleep 2
  FINAL=$(curl -sf "${CURL_AUTH[@]}" "$SCHEDULER/jobs/$JOB_ID")
  STATUS=$(echo "$FINAL" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
  [ "$STATUS" = "done" ] && break
  [ "$STATUS" = "failed" ] && fail "job failed: $FINAL"
done
echo "$FINAL" | grep -q '"status":"done"' || fail "job did not complete"
ok "job done"

echo ""
echo "[7/9] Buckets"
BUCKET="smoke-bucket-$(date +%s)"
curl -sf "${CURL_AUTH[@]}" -X PUT "$STORE/buckets/$BUCKET" || fail "bucket create"
ok "create bucket"
curl -sf "${CURL_AUTH[@]}" -X PUT "$STORE/buckets/$BUCKET/objects/test.txt" -d "bucket hello" || fail "bucket PUT"
ok "bucket PUT"
BBODY=$(curl -sf "${CURL_AUTH[@]}" "$STORE/buckets/$BUCKET/objects/test.txt")
[ "$BBODY" = "bucket hello" ] || fail "bucket GET mismatch: $BBODY"
ok "bucket GET"

echo ""
echo "[8/9] Instance launch"
INST=$(curl -sf "${CURL_AUTH[@]}" -X POST "$REGISTRY/instances")
INST_ID=$(echo "$INST" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
[ -n "$INST_ID" ] || fail "instance launch"
ok "launch $INST_ID"

echo ""
echo "[9/9] Instance-bound job"
INST_NODE=$(echo "$INST" | grep -o '"node_id":"[^"]*"' | cut -d'"' -f4)
BJOB=$(curl -sf "${CURL_AUTH[@]}" -X POST "$SCHEDULER/jobs" \
  -H "Content-Type: application/json" \
  -d "{\"command\":\"echo bound\",\"instance_id\":\"$INST_ID\"}")
BJOB_ID=$(echo "$BJOB" | grep -o '"job_id":"[^"]*"' | cut -d'"' -f4)
BJOB_NODE=$(echo "$BJOB" | grep -o '"node_id":"[^"]*"' | cut -d'"' -f4)
[ -n "$BJOB_ID" ] || fail "bound job missing job_id"
[ "$BJOB_NODE" = "$INST_NODE" ] || fail "bound job wrong node: $BJOB_NODE vs $INST_NODE"

for i in $(seq 1 15); do
  sleep 2
  BFINAL=$(curl -sf "${CURL_AUTH[@]}" "$SCHEDULER/jobs/$BJOB_ID")
  BSTATUS=$(echo "$BFINAL" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
  [ "$BSTATUS" = "done" ] && break
  [ "$BSTATUS" = "failed" ] && fail "bound job failed"
done
echo "$BFINAL" | grep -q '"status":"done"' || fail "bound job did not complete"
ok "instance-bound job done"

echo ""
echo "ALL CHECKS PASSED"
