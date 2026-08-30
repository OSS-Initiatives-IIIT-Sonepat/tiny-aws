# Full-stack demo
# Starts all services, runs the smoke test, then submits a sample job.
# Run from repo root: .\examples\full-stack\demo.ps1

$ErrorActionPreference = "Stop"
$root = $PSScriptRoot + "\..\..\"

Write-Host "=== tiny-aws full-stack demo ==="
Write-Host ""

# 1. Start registry
Write-Host "[1] Starting registry..."
Start-Process go -ArgumentList "run","." -WorkingDirectory "$root\control-plane\registry"
Start-Sleep -Seconds 3

# 2. Start ec2-agent
Write-Host "[2] Starting ec2-agent..."
Start-Process cargo -ArgumentList "run" -WorkingDirectory "$root\data-plane\compute\ec2-agent"
Start-Sleep -Seconds 5

# 3. Start object-store
Write-Host "[3] Starting object-store..."
Start-Process cargo -ArgumentList "run" -WorkingDirectory "$root\data-plane\storage\object-store"
Start-Sleep -Seconds 8

# 4. Start scheduler
Write-Host "[4] Starting scheduler..."
Start-Process go -ArgumentList "run","." -WorkingDirectory "$root\control-plane\scheduler"
Start-Sleep -Seconds 3

Write-Host ""
Write-Host "Stack running. Running smoke test..."
& "$root\tests\integration\smoke-test.ps1"

Write-Host ""
Write-Host "Submitting demo job..."
$job = Invoke-RestMethod -Uri "http://127.0.0.1:9001/jobs" -Method Post `
  -ContentType "application/json" -Body '{"command":"echo hello from tiny-aws"}'
Write-Host "job $($job.job_id) submitted to node $($job.node_id)"

Write-Host ""
Write-Host "Full-stack demo complete."
