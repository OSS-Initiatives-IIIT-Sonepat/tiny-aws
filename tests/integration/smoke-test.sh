#!/usr/bin/env bash
# Integration smoke test for tiny-aws (Linux/macOS).
# Assumes the full stack is already running.
# Run from repo root: bash tests/integration/smoke-test.sh

set -euo pipefail

API_KEY="${TINYAWS_API_KEY:-}"
AUTH_HEADER=""
CURL_AUTH=()
if [ -n "$API_KEY" ]; then
  AUTH_HEADER="Authorization: Bearer $API_KEY"
  CURL_AUTH=(-H "$AUTH_HEADER")
fi

fail() { echo " FAIL"; echo "$1" >&2; exit 1; }

test_endpoint() {
  local name="$1" url="$2" pattern="${3:-.}"
  printf "  checking %s..." "$name"
  local resp
  resp=$(curl -sf "${CURL_AUTH[@]+"${CURL_AUTH[@]}"}" "$url") || fail "$name unreachable: $url"
  echo "$resp" | grep -qE "$pattern" || fail "$name unexpected response: $resp"
  echo " ok"
}

echo "tiny-aws integration smoke test"
echo ""

echo "[1/10] Service health"
test_endpoint "registry"     "http://127.0.0.1:9000/health" '"status":"healthy"'
test_endpoint "ec2-agent"    "http://127.0.0.1:8080/health" '"status":"healthy"'
test_endpoint "object-store" "http://127.0.0.1:7001/health" '"status":"healthy"'
test_endpoint "scheduler"    "http://127.0.0.1:9001/health" '"status":"healthy"'

echo ""
echo "[2/10] Registry nodes"
test_endpoint "nodes"         "http://127.0.0.1:9000/nodes"              '"id"'
test_endpoint "compute nodes" "http://127.0.0.1:9000/nodes?role=compute" '"id"'
test_endpoint "storage nodes" "http://127.0.0.1:9000/nodes?role=storage" "."

echo ""
echo "[3/10] Object store"
object_key="smoke-test-$(date +%H%M%S)"
curl -sf "${CURL_AUTH[@]+"${CURL_AUTH[@]}"}" -X PUT "http://127.0.0.1:7001/objects/$object_key" -d "hello integration" >/dev/null || fail "object PUT failed"
echo "  put object ok"

body=$(curl -sf "${CURL_AUTH[@]+"${CURL_AUTH[@]}"}" "http://127.0.0.1:7001/objects/$object_key")
[ "$body" = "hello integration" ] || fail "object GET mismatch: $body"
echo "  get object ok"

meta=$(curl -sf "${CURL_AUTH[@]+"${CURL_AUTH[@]}"}" "http://127.0.0.1:7001/objects/$object_key/meta")
echo "$meta" | grep -q "$object_key" || fail "object meta missing key"
echo "  object meta ok"

echo ""
echo "[4/10] Scheduler"
sched_ok=false
for i in $(seq 1 10); do
  resp=$(curl -s "${CURL_AUTH[@]+"${CURL_AUTH[@]}"}" "http://127.0.0.1:9001/schedule" 2>/dev/null) || true
  if echo "$resp" | grep -q '"node_id"'; then sched_ok=true; break; fi
  sleep 2
done
$sched_ok || fail "schedule: no healthy compute nodes after 20s"
echo "  schedule ok"

echo ""
echo "[5/10] Job submission"
job_response=$(curl -sf "${CURL_AUTH[@]+"${CURL_AUTH[@]}"}" -X POST -H "Content-Type: application/json" \
  "http://127.0.0.1:9001/jobs" -d '{"command":"echo hello"}')
job_id=$(echo "$job_response" | grep -o '"job_id":"[^"]*"' | head -1 | cut -d'"' -f4)
node_id=$(echo "$job_response" | grep -o '"node_id":"[^"]*"' | head -1 | cut -d'"' -f4)
[ -n "$job_id" ]  || fail "job response missing job_id"
[ -n "$node_id" ] || fail "job response missing node_id"
echo "  submit job ok"

echo ""
echo "[6/10] Job execution"
final_status=""
for i in $(seq 1 15); do
  sleep 2
  job_data=$(curl -sf "${CURL_AUTH[@]+"${CURL_AUTH[@]}"}" "http://127.0.0.1:9001/jobs/$job_id") || continue
  status=$(echo "$job_data" | grep -o '"status":"[^"]*"' | head -1 | cut -d'"' -f4)
  if [ "$status" = "done" ]; then final_status="done"; break; fi
  if [ "$status" = "failed" ]; then fail "job failed: $job_data"; fi
done
[ "$final_status" = "done" ] || fail "job did not complete in time (status=$status)"
echo "  job completed ok"

echo ""
echo "[7/10] Buckets"
bucket="smoke-bucket-$(date +%H%M%S)"
curl -sf "${CURL_AUTH[@]+"${CURL_AUTH[@]}"}" -X PUT "http://127.0.0.1:7001/buckets/$bucket" >/dev/null || fail "bucket create failed"
echo "  create bucket ok"

curl -sf "${CURL_AUTH[@]+"${CURL_AUTH[@]}"}" -X PUT "http://127.0.0.1:7001/buckets/$bucket/objects/test.txt" -d "bucket hello" >/dev/null || fail "bucket object PUT failed"
echo "  put bucket object ok"

bucket_body=$(curl -sf "${CURL_AUTH[@]+"${CURL_AUTH[@]}"}" "http://127.0.0.1:7001/buckets/$bucket/objects/test.txt")
[ "$bucket_body" = "bucket hello" ] || fail "bucket GET mismatch: $bucket_body"
echo "  get bucket object ok"

echo ""
echo "[8/10] Instances"
inst=$(curl -sf "${CURL_AUTH[@]+"${CURL_AUTH[@]}"}" -X POST "http://127.0.0.1:9000/instances")
inst_id=$(echo "$inst" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
inst_node=$(echo "$inst" | grep -o '"node_id":"[^"]*"' | head -1 | cut -d'"' -f4)
[ -n "$inst_id" ] || fail "instance launch failed"
echo "  launch instance ok"

echo ""
echo "[9/10] Instance-bound job"
bound_resp=$(curl -sf "${CURL_AUTH[@]+"${CURL_AUTH[@]}"}" -X POST -H "Content-Type: application/json" \
  "http://127.0.0.1:9001/jobs" -d "{\"command\":\"echo instance-bound\",\"instance_id\":\"$inst_id\"}")
bound_job_id=$(echo "$bound_resp" | grep -o '"job_id":"[^"]*"' | head -1 | cut -d'"' -f4)
bound_node=$(echo "$bound_resp" | grep -o '"node_id":"[^"]*"' | head -1 | cut -d'"' -f4)
[ -n "$bound_job_id" ] || fail "bound job missing job_id"
[ "$bound_node" = "$inst_node" ] || fail "bound job assigned to wrong node: $bound_node expected $inst_node"

bound_final=""
for i in $(seq 1 15); do
  sleep 2
  bdata=$(curl -sf "${CURL_AUTH[@]+"${CURL_AUTH[@]}"}" "http://127.0.0.1:9001/jobs/$bound_job_id") || continue
  bstatus=$(echo "$bdata" | grep -o '"status":"[^"]*"' | head -1 | cut -d'"' -f4)
  if [ "$bstatus" = "done" ]; then bound_final="done"; break; fi
  if [ "$bstatus" = "failed" ]; then fail "bound job failed: $bdata"; fi
done
[ "$bound_final" = "done" ] || fail "bound job did not complete (status=$bstatus)"
echo "  instance-bound job ok"

echo ""
echo "[10/10] Instance workspace"
ws_resp=$(curl -sf "${CURL_AUTH[@]+"${CURL_AUTH[@]}"}" -X POST -H "Content-Type: application/json" \
  "http://127.0.0.1:9001/jobs" -d "{\"command\":\"echo __WS__\",\"instance_id\":\"$inst_id\"}")
ws_job_id=$(echo "$ws_resp" | grep -o '"job_id":"[^"]*"' | head -1 | cut -d'"' -f4)
[ -n "$ws_job_id" ] || fail "workspace job missing job_id"

ws_final=""
for i in $(seq 1 15); do
  sleep 2
  wdata=$(curl -sf "${CURL_AUTH[@]+"${CURL_AUTH[@]}"}" "http://127.0.0.1:9001/jobs/$ws_job_id") || continue
  wstatus=$(echo "$wdata" | grep -o '"status":"[^"]*"' | head -1 | cut -d'"' -f4)
  if [ "$wstatus" = "done" ]; then ws_final="done"; break; fi
  if [ "$wstatus" = "failed" ]; then fail "workspace job failed: $wdata"; fi
done
[ "$ws_final" = "done" ] || fail "workspace job did not complete"
echo "  workspace job ok"

echo ""
echo "ALL CHECKS PASSED"
