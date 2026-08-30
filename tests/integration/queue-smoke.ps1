# Queue (SQS) integration smoke test for tiny-aws.
# Requires SQS service running: cd control-plane/messaging/sqs && go run .
# Run from repo root: .\tests\integration\queue-smoke.ps1

$ErrorActionPreference = "Stop"

$sqs = "http://127.0.0.1:9003"
$queue = "smoke-q-$(Get-Date -Format 'HHmmss')"

Write-Host "queue integration smoke test"
Write-Host ""

Write-Host "[1/4] SQS health"
$h = Invoke-RestMethod "$sqs/health"
if ($h.status -ne "healthy") { throw "sqs not healthy" }
Write-Host "  healthy"

Write-Host "[2/4] Create queue"
Invoke-RestMethod -Uri "$sqs/queues/$queue" -Method Post | Out-Null
Write-Host "  created $queue"

Write-Host "[3/4] Send message"
$sent = Invoke-RestMethod -Uri "$sqs/queues/$queue/messages" -Method Post -ContentType "application/json" -Body '{"body":"hello-queue"}'
if (-not $sent.id) { throw "send missing id" }
Write-Host "  sent $($sent.id)"

Write-Host "[4/4] Receive and ack message"
$msg = Invoke-RestMethod "$sqs/queues/$queue/messages"
if ($msg.body -ne "hello-queue") { throw "receive mismatch: $($msg.body)" }
Invoke-RestMethod -Uri "$sqs/queues/$queue/messages/$($msg.id)" -Method Delete | Out-Null
Write-Host "  received and acked"

Write-Host ""
Write-Host "QUEUE SMOKE TEST PASSED"
