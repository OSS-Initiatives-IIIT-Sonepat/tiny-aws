# Controller integration smoke test.
# Requires: controller running (CONTROLLER_ADDR=:9002 go run . in control-plane/controller)
# Run from repo root: .\tests\integration\controller-smoke.ps1

$ErrorActionPreference = "Stop"

$controller = if ($env:CONTROLLER_URL) { $env:CONTROLLER_URL } else { "http://127.0.0.1:9002" }

Write-Host "controller smoke test"
Write-Host ""

Write-Host "[1/2] Health"
$resp = curl.exe -s -f "$controller/health"
if ($LASTEXITCODE -ne 0) { throw "controller not reachable" }
if ($resp -notmatch "healthy") { throw "controller not healthy: $resp" }
Write-Host "  controller ok"

Write-Host "[2/2] Manual reconcile trigger"
$result = curl.exe -s -f -X POST "$controller/reconcile"
if ($LASTEXITCODE -ne 0) { throw "reconcile request failed" }
if ($result -notmatch "reconciled") { throw "reconcile response unexpected: $result" }
Write-Host "  reconcile triggered ok"

Write-Host ""
Write-Host "CONTROLLER SMOKE TEST PASSED"
