#!/bin/bash
# Start the full tiny-aws stack on Linux.
# Run from repo root: ./scripts/run-local.sh

set -e
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

echo "Starting tiny-aws stack..."
echo "Order: registry -> ec2-agent -> object-store -> scheduler"
echo ""

# registry
(cd "$ROOT/control-plane/registry" && go run . ) &
REGISTRY_PID=$!
echo "registry started (pid=$REGISTRY_PID)"
sleep 3

# ec2-agent
(cd "$ROOT/data-plane/compute/ec2-agent" && cargo run ) &
AGENT_PID=$!
echo "ec2-agent started (pid=$AGENT_PID)"
sleep 5

# object-store
(cd "$ROOT/data-plane/storage/object-store" && cargo run ) &
STORE_PID=$!
echo "object-store started (pid=$STORE_PID)"
sleep 8

# scheduler
(cd "$ROOT/control-plane/scheduler" && go run . ) &
SCHED_PID=$!
echo "scheduler started (pid=$SCHED_PID)"
sleep 3

echo ""
echo "Stack running. PIDs: registry=$REGISTRY_PID agent=$AGENT_PID store=$STORE_PID scheduler=$SCHED_PID"
echo ""
echo "Health:"
echo "  registry      http://127.0.0.1:9000/health"
echo "  ec2-agent     http://127.0.0.1:8080/health"
echo "  object-store  http://127.0.0.1:7001/health"
echo "  scheduler     http://127.0.0.1:9001/health"
echo ""
echo "Run smoke test: ./tests/integration/service-smoke.sh"
echo "Stop stack:     kill $REGISTRY_PID $AGENT_PID $STORE_PID $SCHED_PID"

wait
