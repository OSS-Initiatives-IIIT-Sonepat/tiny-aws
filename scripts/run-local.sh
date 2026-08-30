#!/bin/bash
# Start the full tiny-aws stack on Linux.
# Run from repo root: ./scripts/run-local.sh
# Optionally set env vars in .env.local before running.

set -e
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

# source .env.local if it exists
if [ -f "$ROOT/.env.local" ]; then
  set -a
  # shellcheck disable=SC1091
  source "$ROOT/.env.local"
  set +a
  echo "loaded .env.local"
fi

echo "Building Rust crates first (this may take a few minutes on first run)..."
(cd "$ROOT/data-plane/compute/ec2-agent" && cargo build 2>&1 | tail -1)
(cd "$ROOT/data-plane/storage/object-store" && cargo build 2>&1 | tail -1)
echo "Rust build done."
echo ""

echo "Starting tiny-aws stack..."
PIDS=()

# 1. registry
(cd "$ROOT/control-plane/registry" && go run . ) &
PIDS+=($!)
echo "registry started (pid=$!)"
sleep 2

# 2. ec2-agent (pre-built)
(cd "$ROOT/data-plane/compute/ec2-agent" && ./target/debug/ec2-agent) &
PIDS+=($!)
echo "ec2-agent started (pid=$!)"
sleep 3

# 3. object-store (pre-built)
(cd "$ROOT/data-plane/storage/object-store" && ./target/debug/object-store) &
PIDS+=($!)
echo "object-store started (pid=$!)"
sleep 5

# 4. scheduler
(cd "$ROOT/control-plane/scheduler" && go run . ) &
PIDS+=($!)
echo "scheduler started (pid=$!)"
sleep 2

# optional services — start if binaries/modules exist
# controller
(cd "$ROOT/control-plane/controller" && go run . ) &
PIDS+=($!)
echo "controller started"

# load balancer
(cd "$ROOT/data-plane/networking/load-balancer" && go run . ) &
PIDS+=($!)
echo "load-balancer started"

echo ""
echo "Stack running. PIDs: ${PIDS[*]}"
echo ""
echo "Health:"
echo "  registry      $REGISTRY_URL"
echo "  ec2-agent     http://127.0.0.1:8080/health"
echo "  object-store  ${OBJECT_STORE_URL:-http://127.0.0.1:7001}/health"
echo "  scheduler     ${SCHEDULER_URL:-http://127.0.0.1:9001}/health"
echo ""
echo "Run smoke test: ./tests/integration/smoke-test.sh"
echo "Stop stack:     kill ${PIDS[*]}"

# write PIDs for easy cleanup
printf '%s\n' "${PIDS[@]}" > /tmp/tinyaws-pids
echo "(PIDs saved to /tmp/tinyaws-pids — stop with: kill \$(cat /tmp/tinyaws-pids))"

wait
