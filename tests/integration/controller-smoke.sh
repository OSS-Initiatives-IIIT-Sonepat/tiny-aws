#!/bin/bash
# Controller integration smoke test.
# Requires: controller running (CONTROLLER_ADDR=:9002 go run . in control-plane/controller)
# Run from repo root: ./tests/integration/controller-smoke.sh

set -e

CONTROLLER=${CONTROLLER_URL:-http://127.0.0.1:9002}
ok()   { echo "  $1 ok"; }
fail() { echo "FAIL: $1"; exit 1; }

echo "controller smoke test"
echo ""

echo "[1/2] Health"
curl -sf "$CONTROLLER/health" | grep -q healthy || fail "controller not healthy"
ok "controller"

echo "[2/2] Manual reconcile trigger"
RESULT=$(curl -sf -X POST "$CONTROLLER/reconcile")
echo "$RESULT" | grep -q "reconciled" || fail "reconcile response unexpected: $RESULT"
ok "reconcile triggered"

echo ""
echo "CONTROLLER SMOKE TEST PASSED"
