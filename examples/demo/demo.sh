#!/usr/bin/env bash
# tiny-aws demo: shows the full platform working on a single Linux box.
# Prerequisites: all services running (./scripts/tinyaws-start.sh)
set -euo pipefail

echo "=== tiny-aws demo ==="
echo ""

# 1. Show cluster status
echo "--- Step 1: Cluster status ---"
tinyaws node list --healthy-only
echo ""

# 2. Launch instances with different sizes
echo "--- Step 2: Launch instances ---"
tinyaws instance launch --type nano
tinyaws instance launch --type small
tinyaws instance launch --type medium
echo ""
sleep 3
tinyaws instance list
echo ""

# 3. Deploy a simple web service
echo "--- Step 3: Deploy a web service ---"
DEMO_DIR=$(mktemp -d)
cat > "$DEMO_DIR/app.js" << 'EOF'
const http = require('http');
const port = process.env.PORT || 3000;
http.createServer((req, res) => {
  res.writeHead(200, {'Content-Type': 'application/json'});
  res.end(JSON.stringify({
    service: 'tiny-aws-demo',
    timestamp: new Date().toISOString(),
    env: { NODE_ENV: process.env.NODE_ENV || 'development' }
  }));
}).listen(port, () => console.log(`listening on ${port}`));
EOF
cat > "$DEMO_DIR/start.sh" << 'EOF'
#!/bin/sh
node app.js
EOF
chmod +x "$DEMO_DIR/start.sh"

tinyaws deploy "$DEMO_DIR" --service --port 3000 --env PORT=3000 --env NODE_ENV=production
echo ""
sleep 5

# 4. Store an object
echo "--- Step 4: Object storage ---"
tinyaws bucket create demo-bucket
tinyaws object put hello.txt --bucket demo-bucket --data "Hello from tiny-aws!"
tinyaws object get hello.txt --bucket demo-bucket
echo ""

# 5. Send a message through SQS
echo "--- Step 5: Message queue ---"
tinyaws queue create demo-queue
tinyaws queue send demo-queue "Hello from the queue!"
tinyaws queue receive demo-queue
echo ""

# 6. Run a command inside an instance
echo "--- Step 6: Execute in instance ---"
INST=$(tinyaws instance list | head -1 | awk '{print $1}')
if [ -n "$INST" ]; then
    tinyaws exec "$INST" -- cat /proc/meminfo | head -3
fi
echo ""

# 7. Check services
echo "--- Step 7: Running services ---"
tinyaws service list
echo ""

# Cleanup
rm -rf "$DEMO_DIR"

echo "=== Demo complete ==="
echo "tiny-aws: a working mini-cloud on one Linux box."
echo "No Docker. No Kubernetes. Just Go, Rust, C++, and Linux syscalls."
