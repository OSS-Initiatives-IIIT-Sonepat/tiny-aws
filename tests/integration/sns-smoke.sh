#!/bin/bash
# SNS integration smoke test.
# Requires: SNS service running (SNS_ADDR=:9004 go run . in control-plane/messaging/sns)
# Run from repo root: ./tests/integration/sns-smoke.sh

set -e

SNS=${SNS_URL:-http://127.0.0.1:9004}
ok()   { echo "  $1 ok"; }
fail() { echo "FAIL: $1"; exit 1; }

echo "SNS smoke test"
echo ""

echo "[1/5] Health"
curl -sf "$SNS/health" | grep -q healthy || fail "sns not healthy"
ok "sns"

echo "[2/5] Create topic"
TOPIC="smoke-topic-$(date +%s)"
curl -sf -X POST "$SNS/topics/$TOPIC" | grep -q "$TOPIC" || fail "topic create"
ok "topic $TOPIC created"

echo "[3/5] List topics"
curl -sf "$SNS/topics" | grep -q "$TOPIC" || fail "topic not in list"
ok "topic in list"

echo "[4/5] Subscribe endpoint"
# use a local netcat listener as the subscriber endpoint
# ponytail: just verify the subscription is stored, not actual delivery
SUB=$(curl -sf -X POST "$SNS/topics/$TOPIC/subscribe" \
  -H "Content-Type: application/json" \
  -d '{"endpoint":"http://127.0.0.1:19998/notify"}')
SUB_ID=$(echo "$SUB" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
[ -n "$SUB_ID" ] || fail "subscribe missing id: $SUB"
ok "subscribed $SUB_ID"

echo "[5/5] Publish to topic"
RESULT=$(curl -sf -X POST "$SNS/topics/$TOPIC/publish" \
  -H "Content-Type: application/json" \
  -d '{"message":"hello-sns"}')
# returns {"delivered": N} — delivery to unreachable endpoint logs and continues
echo "$RESULT" | grep -q "delivered" || fail "publish response missing delivered: $RESULT"
ok "published (delivered field present)"

echo ""
echo "SNS SMOKE TEST PASSED"
