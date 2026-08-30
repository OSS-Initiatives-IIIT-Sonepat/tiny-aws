#!/bin/bash
# API Gateway integration smoke test.
# Verifies that the gateway correctly routes /v1/* to backend services.
# Requires: registry + scheduler + object-store + api-gateway running.
# Run from repo root: ./tests/integration/gateway-smoke.sh

set -e

GW=${TINYAWS_API_URL:-http://127.0.0.1:8000}
REGISTRY=${REGISTRY_URL:-http://127.0.0.1:9000}

CURL_AUTH=()
if [ -n "$TINYAWS_API_KEY" ]; then
  CURL_AUTH=(-H "Authorization: Bearer $TINYAWS_API_KEY")
fi

ok()   { echo "  $1 ok"; }
fail() { echo "FAIL: $1"; exit 1; }

echo "API gateway smoke test"
echo ""

echo "[1/6] Gateway health endpoints"
curl -sf "${CURL_AUTH[@]}" "$GW/v1/health/registry"  | grep -q healthy || fail "/v1/health/registry"
ok "/v1/health/registry"
curl -sf "${CURL_AUTH[@]}" "$GW/v1/health/scheduler" | grep -q healthy || fail "/v1/health/scheduler"
ok "/v1/health/scheduler"
curl -sf "$GW/v1/health/store" | grep -q healthy || fail "/v1/health/store"
ok "/v1/health/store"

echo ""
echo "[2/6] /v1/nodes routes to registry"
NODES=$(curl -sf "${CURL_AUTH[@]}" "$GW/v1/nodes")
echo "$NODES" | grep -q "{" || fail "/v1/nodes returned nothing"
ok "/v1/nodes"

echo ""
echo "[3/6] /v1/jobs routes to scheduler"
JOBS=$(curl -sf "${CURL_AUTH[@]}" "$GW/v1/jobs")
# jobs returns a JSON array
echo "$JOBS" | grep -q "\[" || fail "/v1/jobs returned nothing"
ok "/v1/jobs"

echo ""
echo "[4/6] /v1/objects routes to object-store (PUT then GET)"
GW_KEY="gw-smoke-$(date +%s)"
curl -sf "${CURL_AUTH[@]}" -X PUT "$GW/v1/objects/$GW_KEY" -d "gateway-test" || fail "/v1/objects PUT"
GBODY=$(curl -sf "${CURL_AUTH[@]}" "$GW/v1/objects/$GW_KEY")
[ "$GBODY" = "gateway-test" ] || fail "/v1/objects GET mismatch: $GBODY"
ok "/v1/objects PUT+GET"

echo ""
echo "[5/6] /v1/instances routes to registry"
# verify we can POST (launch) through gateway
INST=$(curl -sf "${CURL_AUTH[@]}" -X POST "$GW/v1/instances" \
  -H "Content-Type: application/json" -d '{"instance_type":"small"}')
INST_ID=$(echo "$INST" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
[ -n "$INST_ID" ] || fail "/v1/instances POST returned no id: $INST"
ok "/v1/instances POST → $INST_ID"

echo ""
echo "[6/6] Compare direct vs gateway response"
# GET the same instance directly and via gateway — should match
DIRECT=$(curl -sf "${CURL_AUTH[@]}" "$REGISTRY/instances/$INST_ID")
VIA_GW=$(curl -sf "${CURL_AUTH[@]}" "$GW/v1/instances/$INST_ID")
[ "$DIRECT" = "$VIA_GW" ] || fail "direct vs gateway response mismatch"
ok "gateway response matches direct"

echo ""
echo "GATEWAY SMOKE TEST PASSED"
