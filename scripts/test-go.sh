#!/bin/bash
# Run all Go unit tests across the repository.
# Usage: ./scripts/test-go.sh

set -e

MODULES=(
    "control-plane/registry"
    "control-plane/scheduler"
    "control-plane/messaging/sqs"
    "control-plane/messaging/sns"
    "control-plane/networking/vpc"
    "control-plane/controller"
    "control-plane/api"
    "control-plane/metadata"
    "control-plane/cli"
    "data-plane/compute/lambda-runtime"
    "data-plane/networking/load-balancer"
)

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PASS=0
FAIL=0

for mod in "${MODULES[@]}"; do
    echo "=== testing $mod ==="
    if (cd "$ROOT/$mod" && go test -count=1 ./...); then
        ((PASS++))
    else
        ((FAIL++))
    fi
    echo
done

echo "=== $PASS passed, $FAIL failed ==="
[ "$FAIL" -eq 0 ]
