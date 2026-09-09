#!/usr/bin/env bash
# Starts all tiny-aws services in background.
# Source .env.local if present for custom config.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

if [ -f "$ROOT_DIR/.env.local" ]; then
    set -a; source "$ROOT_DIR/.env.local"; set +a
fi

LOG_DIR="${TINYAWS_LOG_DIR:-/var/log/tinyaws}"
mkdir -p "$LOG_DIR"

echo "starting tiny-aws services..."

# control plane
cd "$ROOT_DIR/control-plane/registry" && go run . > "$LOG_DIR/registry.log" 2>&1 &
sleep 1
cd "$ROOT_DIR/control-plane/scheduler" && go run . > "$LOG_DIR/scheduler.log" 2>&1 &
cd "$ROOT_DIR/control-plane/controller" && go run . > "$LOG_DIR/controller.log" 2>&1 &
cd "$ROOT_DIR/control-plane/messaging/sqs" && go run . > "$LOG_DIR/sqs.log" 2>&1 &
cd "$ROOT_DIR/control-plane/messaging/sns" && go run . > "$LOG_DIR/sns.log" 2>&1 &
cd "$ROOT_DIR/control-plane/networking" && go run . > "$LOG_DIR/networking.log" 2>&1 &
cd "$ROOT_DIR/control-plane/metadata" && go run . > "$LOG_DIR/metadata.log" 2>&1 &

# data plane
cd "$ROOT_DIR/data-plane/storage/object-store" && cargo run --release > "$LOG_DIR/object-store.log" 2>&1 &
sleep 2
cd "$ROOT_DIR/data-plane/compute/ec2-agent" && cargo run --release > "$LOG_DIR/ec2-agent.log" 2>&1 &
cd "$ROOT_DIR/data-plane/compute/lambda-runtime" && go run . > "$LOG_DIR/lambda.log" 2>&1 &
cd "$ROOT_DIR/data-plane/networking/load-balancer" && go run . > "$LOG_DIR/lb.log" 2>&1 &

# api gateway
cd "$ROOT_DIR/control-plane/api" && go run . > "$LOG_DIR/api-gateway.log" 2>&1 &

echo "all services started. logs in $LOG_DIR"
