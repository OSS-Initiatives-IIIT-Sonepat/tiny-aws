#!/bin/bash
# Deploy a k3s Kubernetes cluster on top of tiny-aws instances.
#
# Creates:
#   i-server  — k3s server (control plane), instance type: large
#   i-worker1 — k3s agent (worker), instance type: medium
#   i-worker2 — k3s agent (worker), instance type: medium
#
# Requirements:
#   - tiny-aws stack running (./scripts/run-local.sh)
#   - base rootfs bootstrapped (sudo ./scripts/bootstrap-rootfs.sh)
#   - tinyaws CLI on PATH
#
# Usage:
#   ./examples/kubernetes/deploy-cluster.sh
#   ./examples/kubernetes/deploy-cluster.sh --workers 3

set -e

WORKERS="${2:-2}"
CLI="${TINYAWS_CLI:-tinyaws}"
REGISTRY_URL="${REGISTRY_URL:-http://127.0.0.1:9000}"
SCHEDULER_URL="${SCHEDULER_URL:-http://127.0.0.1:9001}"

AUTH_HEADER=""
if [ -n "$TINYAWS_API_KEY" ]; then
  AUTH_HEADER="-H Authorization: Bearer $TINYAWS_API_KEY"
fi

ok()   { echo "  ✓ $1"; }
info() { echo "  → $1"; }
fail() { echo "FAIL: $1"; exit 1; }

echo "═══════════════════════════════════════"
echo "  tiny-aws × k3s cluster deploy"
echo "  workers: $WORKERS"
echo "═══════════════════════════════════════"
echo ""

# ── 1. Launch server instance ────────────────────────────────────────────────
echo "[1/5] Launching k3s server instance (large)..."
SERVER_RESP=$($CLI instance launch --type large)
SERVER_ID=$(echo "$SERVER_RESP" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
[ -n "$SERVER_ID" ] || fail "server instance launch failed: $SERVER_RESP"
ok "server instance: $SERVER_ID"

# wait for provisioning
info "waiting for $SERVER_ID to provision..."
for i in $(seq 1 30); do
  sleep 3
  STATUS=$(curl -sf $AUTH_HEADER "$REGISTRY_URL/instances/$SERVER_ID" \
    | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
  [ "$STATUS" = "running" ] && break
  [ "$STATUS" = "failed" ] && fail "$SERVER_ID provisioning failed"
  info "  $STATUS..."
done
[ "$STATUS" = "running" ] || fail "$SERVER_ID did not reach running in 90s"
ok "$SERVER_ID is running"

# ── 2. Install k3s server ────────────────────────────────────────────────────
echo ""
echo "[2/5] Installing k3s server on $SERVER_ID..."

mkdir -p /tmp/tinyaws-k3s
cat > /tmp/tinyaws-k3s/start.sh << 'SERVERSCRIPT'
#!/bin/bash
set -e
echo "installing k3s server..."
curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="server --disable traefik" sh -
echo "k3s server installed"
# print node token for workers to join
echo "NODE_TOKEN=$(cat /var/lib/rancher/k3s/server/node-token)"
# keep running
exec journalctl -u k3s -f
SERVERSCRIPT
chmod +x /tmp/tinyaws-k3s/start.sh

SERVER_JOB=$($CLI deploy /tmp/tinyaws-k3s --service --port 6443 --instance "$SERVER_ID")
SERVER_JOB_ID=$(echo "$SERVER_JOB" | grep -o 'job-[0-9]*' | head -1)
ok "k3s server deploy job: $SERVER_JOB_ID"

# wait for k3s to start (check service list for port 6443)
info "waiting for k3s server to start (this takes ~30s)..."
sleep 30
for i in $(seq 1 20); do
  sleep 3
  SVC_STATUS=$(curl -sf $AUTH_HEADER "$REGISTRY_URL/services" \
    | grep -o '"port":6443' | head -1)
  [ -n "$SVC_STATUS" ] && break
  info "  waiting..."
done
ok "k3s server is up on port 6443"

# ── 3. Get join token from the container ────────────────────────────────────
echo ""
echo "[3/5] Fetching join token..."
TOKEN=$(sudo machinectl shell "$SERVER_ID" /bin/bash -c \
  'cat /var/lib/rancher/k3s/server/node-token' 2>/dev/null || echo "")

if [ -z "$TOKEN" ]; then
  info "Could not auto-fetch token. Get it manually:"
  info "  sudo machinectl shell $SERVER_ID"
  info "  cat /var/lib/rancher/k3s/server/node-token"
  read -rp "Paste the token here: " TOKEN
fi
[ -n "$TOKEN" ] || fail "no join token"
ok "token: ${TOKEN:0:20}..."

# get server IP (use advertised addr if set, else localhost)
SERVER_IP="${AGENT_ADVERTISE_ADDR:-127.0.0.1}"
SERVER_URL="https://$SERVER_IP:6443"

# ── 4. Launch and join worker instances ─────────────────────────────────────
echo ""
echo "[4/5] Launching $WORKERS worker instances (medium)..."

WORKER_IDS=()
for i in $(seq 1 "$WORKERS"); do
  WRESP=$($CLI instance launch --type medium)
  WID=$(echo "$WRESP" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
  [ -n "$WID" ] || fail "worker $i launch failed"
  WORKER_IDS+=("$WID")
  ok "worker $i: $WID"
done

# wait for all workers to provision
info "waiting for workers to provision..."
for WID in "${WORKER_IDS[@]}"; do
  for i in $(seq 1 30); do
    sleep 3
    WSTATUS=$(curl -sf $AUTH_HEADER "$REGISTRY_URL/instances/$WID" \
      | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
    [ "$WSTATUS" = "running" ] && break
    [ "$WSTATUS" = "failed" ] && fail "$WID provisioning failed"
  done
  ok "$WID is running"
done

# deploy k3s agent on each worker
mkdir -p /tmp/tinyaws-k3s-worker
cat > /tmp/tinyaws-k3s-worker/start.sh << WORKERSCRIPT
#!/bin/bash
set -e
echo "joining k3s cluster at $SERVER_URL ..."
curl -sfL https://get.k3s.io | \\
  K3S_URL="$SERVER_URL" \\
  K3S_TOKEN="$TOKEN" \\
  sh -
echo "k3s agent joined"
exec journalctl -u k3s-agent -f
WORKERSCRIPT
chmod +x /tmp/tinyaws-k3s-worker/start.sh

for WID in "${WORKER_IDS[@]}"; do
  $CLI deploy /tmp/tinyaws-k3s-worker --service --instance "$WID" > /dev/null
  ok "k3s agent deployed on $WID"
done

# ── 5. Summary ───────────────────────────────────────────────────────────────
echo ""
echo "[5/5] Waiting 20s for workers to join..."
sleep 20

echo ""
echo "═══════════════════════════════════════"
echo "  Cluster ready"
echo "═══════════════════════════════════════"
echo ""
echo "Server:  $SERVER_ID  (port 6443)"
printf "Workers: %s\n" "${WORKER_IDS[@]}"
echo ""
echo "Access the cluster:"
echo "  sudo machinectl shell $SERVER_ID"
echo "  kubectl get nodes"
echo ""
echo "Or copy kubeconfig to host:"
echo "  sudo cat /var/lib/tinyaws/instances/$SERVER_ID/etc/rancher/k3s/k3s.yaml"
echo "  # replace 127.0.0.1 with $SERVER_IP"
echo ""
echo "Tear down:"
for WID in "${WORKER_IDS[@]}"; do
  echo "  tinyaws instance terminate $WID"
done
echo "  tinyaws instance terminate $SERVER_ID"
echo ""
echo "Run 'tinyaws instance list' to see all instances."
