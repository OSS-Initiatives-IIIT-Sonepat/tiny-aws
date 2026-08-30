#!/bin/bash
# Lambda invoke integration smoke test.
# Tests the full path: create function → upload code → invoke → verify output.
# Requires: registry + scheduler + ec2-agent + object-store + lambda-runtime running.
# Run from repo root: ./tests/integration/lambda-invoke-smoke.sh

set -e

LAMBDA=${LAMBDA_URL:-http://127.0.0.1:9007}
STORE=${OBJECT_STORE_URL:-http://127.0.0.1:7001}
SCHEDULER=${SCHEDULER_URL:-http://127.0.0.1:9001}

CURL_AUTH=()
if [ -n "$TINYAWS_API_KEY" ]; then
  CURL_AUTH=(-H "Authorization: Bearer $TINYAWS_API_KEY")
fi

ok()   { echo "  $1 ok"; }
fail() { echo "FAIL: $1"; exit 1; }

echo "lambda invoke smoke test"
echo ""

echo "[1/6] Health checks"
curl -sf "${CURL_AUTH[@]}" "$LAMBDA/health"    | grep -q healthy || fail "lambda not healthy"
ok "lambda"
curl -sf "${CURL_AUTH[@]}" "$SCHEDULER/health" | grep -q healthy || fail "scheduler not healthy"
ok "scheduler"
command -v python3 > /dev/null || fail "python3 not on PATH (needed for lambda runtime)"
ok "python3 available"

echo ""
echo "[2/6] Create function zip"
WORK_DIR=$(mktemp -d)
trap 'rm -rf "$WORK_DIR"' EXIT

cat > "$WORK_DIR/handler.py" << 'PYEOF'
import json

def handler(event, context=None):
    name = (event or {}).get("name", "world") if isinstance(event, dict) else "world"
    return {"message": f"Hello, {name}!", "status": "ok"}
PYEOF

(cd "$WORK_DIR" && zip fn.zip handler.py > /dev/null)
ok "handler.py zipped"

echo ""
echo "[3/6] Upload code to object store"
curl -sf "${CURL_AUTH[@]}" -X PUT "$STORE/buckets/lambdas" > /dev/null 2>&1 || true
curl -sf "${CURL_AUTH[@]}" -X PUT "$STORE/buckets/lambdas/objects/smoke-fn.zip" \
  --data-binary @"$WORK_DIR/fn.zip" > /dev/null || fail "upload fn.zip"
ok "code uploaded to lambdas/smoke-fn.zip"

echo ""
echo "[4/6] Register function"
FN_RESP=$(curl -sf "${CURL_AUTH[@]}" -X POST "$LAMBDA/functions" \
  -H "Content-Type: application/json" \
  -d '{"name":"smoke-fn","runtime":"python3","handler":"handler.handler","bucket":"lambdas","key":"smoke-fn.zip"}')
echo "$FN_RESP" | grep -q "smoke-fn" || fail "function create: $FN_RESP"
ok "function registered"

echo ""
echo "[5/6] Verify in list + GET"
curl -sf "${CURL_AUTH[@]}" "$LAMBDA/functions" | grep -q "smoke-fn" || fail "function not in list"
ok "in list"
curl -sf "${CURL_AUTH[@]}" "$LAMBDA/functions/smoke-fn" | grep -q "python3" || fail "GET function"
ok "GET function"

echo ""
echo "[6/6] Invoke function and verify output"
INVOKE_RESP=$(curl -sf "${CURL_AUTH[@]}" -X POST "$LAMBDA/functions/smoke-fn/invoke" \
  -H "Content-Type: application/json" \
  -d '{"name":"tiny-aws"}')

echo "  invoke response: $INVOKE_RESP"

# response is {"status_code":200,"output":"..."}
STATUS_CODE=$(echo "$INVOKE_RESP" | grep -o '"status_code":[0-9]*' | cut -d: -f2)
[ "$STATUS_CODE" = "200" ] || fail "invoke returned status_code=$STATUS_CODE (expected 200)"

OUTPUT=$(echo "$INVOKE_RESP" | grep -o '"output":"[^"]*"' | cut -d'"' -f4)
echo "$OUTPUT" | grep -qi "hello" || fail "invoke output missing 'hello': $OUTPUT"
ok "invoke returned status_code=200 with hello output"

echo ""
echo "LAMBDA INVOKE SMOKE TEST PASSED"
