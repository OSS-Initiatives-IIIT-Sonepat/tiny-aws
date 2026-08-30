#!/bin/bash
# IAM integration smoke test.
# Tests: key create, list, readonly enforcement, expiry, delete.
# Requires: registry running with TINYAWS_API_KEY set.
# Run from repo root: ./tests/integration/iam-smoke.sh

set -e

REGISTRY=${REGISTRY_URL:-http://127.0.0.1:9000}

[ -n "$TINYAWS_API_KEY" ] || { echo "SKIP: TINYAWS_API_KEY not set — IAM enforcement only active with a key"; exit 0; }
ADMIN_AUTH=(-H "Authorization: Bearer $TINYAWS_API_KEY")

ok()   { echo "  $1 ok"; }
fail() { echo "FAIL: $1"; exit 1; }

echo "IAM smoke test"
echo ""

echo "[1/6] Create admin key"
ADMIN_KEY="iam-test-admin-$(date +%s)"
RESP=$(curl -sf "${ADMIN_AUTH[@]}" -X POST "$REGISTRY/iam/keys" \
  -H "Content-Type: application/json" \
  -d "{\"key\":\"$ADMIN_KEY\",\"role\":\"admin\"}")
echo "$RESP" | grep -q "admin" || fail "create admin key: $RESP"
ok "admin key created"

echo "[2/6] Create readonly key"
RO_KEY="iam-test-ro-$(date +%s)"
curl -sf "${ADMIN_AUTH[@]}" -X POST "$REGISTRY/iam/keys" \
  -H "Content-Type: application/json" \
  -d "{\"key\":\"$RO_KEY\",\"role\":\"readonly\"}" | grep -q "readonly" || fail "create readonly key"
ok "readonly key created"

echo "[3/6] List keys — both appear"
KEYS=$(curl -sf "${ADMIN_AUTH[@]}" "$REGISTRY/iam/keys")
echo "$KEYS" | grep -q "$ADMIN_KEY" || fail "admin key not in list"
echo "$KEYS" | grep -q "$RO_KEY"    || fail "readonly key not in list"
ok "both keys in list"

echo "[4/6] Readonly key — GET allowed"
curl -sf -H "Authorization: Bearer $RO_KEY" "$REGISTRY/nodes" > /dev/null || fail "readonly GET /nodes failed"
ok "readonly GET works"

echo "[5/6] Readonly key — POST rejected with 403"
CODE=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "Authorization: Bearer $RO_KEY" \
  -X POST "$REGISTRY/instances" \
  -H "Content-Type: application/json" -d '{}')
[ "$CODE" = "403" ] || fail "readonly POST should be 403, got $CODE"
ok "readonly POST returns 403"

echo "[6/6] Create expiring key — expired key rejected"
EXP_KEY="iam-test-expired-$(date +%s)"
PAST="2020-01-01T00:00:00Z"
curl -sf "${ADMIN_AUTH[@]}" -X POST "$REGISTRY/iam/keys" \
  -H "Content-Type: application/json" \
  -d "{\"key\":\"$EXP_KEY\",\"role\":\"admin\",\"expires_at\":\"$PAST\"}" > /dev/null || fail "create expired key"

CODE=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "Authorization: Bearer $EXP_KEY" \
  "$REGISTRY/nodes")
[ "$CODE" = "401" ] || fail "expired key should return 401, got $CODE"
ok "expired key returns 401"

echo ""
echo "Cleaning up test keys..."
for K in "$ADMIN_KEY" "$RO_KEY" "$EXP_KEY"; do
  curl -sf "${ADMIN_AUTH[@]}" -X DELETE "$REGISTRY/iam/keys/$K" || true
done
ok "keys deleted"

echo ""
echo "IAM SMOKE TEST PASSED"
