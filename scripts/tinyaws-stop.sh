#!/usr/bin/env bash
# Stops all tiny-aws services.
set -euo pipefail

echo "stopping tiny-aws services..."

# kill all Go/Rust processes started by tinyaws-start
pkill -f "control-plane/registry" || true
pkill -f "control-plane/scheduler" || true
pkill -f "control-plane/controller" || true
pkill -f "control-plane/messaging/sqs" || true
pkill -f "control-plane/messaging/sns" || true
pkill -f "control-plane/networking" || true
pkill -f "control-plane/metadata" || true
pkill -f "control-plane/api" || true
pkill -f "data-plane/storage/object-store" || true
pkill -f "data-plane/compute/ec2-agent" || true
pkill -f "data-plane/compute/lambda-runtime" || true
pkill -f "data-plane/networking/load-balancer" || true

echo "all services stopped"
