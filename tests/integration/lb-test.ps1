# Load balancer integration test for tiny-aws.
# Requires: registry + ec2-agent + load-balancer running.
# Run from repo root: .\tests\integration\lb-test.ps1

$ErrorActionPreference = "Stop"

$lb = "http://127.0.0.1:8088"

Write-Host "load balancer integration test"
Write-Host ""

Write-Host "[1/3] LB health"
$h = Invoke-RestMethod "$lb/health"
if ($h.status -ne "healthy") { throw "lb not healthy" }
Write-Host "  healthy, targets=$($h.targets)"

Write-Host "[2/3] LB targets list"
$targets = Invoke-RestMethod "$lb/targets"
Write-Host "  $($targets.Count) target(s) registered"

Write-Host "[3/3] LB proxy to agent /health"
$agentHealth = Invoke-RestMethod "$lb/health"
if (-not $agentHealth.status) { throw "lb proxy returned no status" }
Write-Host "  proxied ok: status=$($agentHealth.status)"

Write-Host ""
Write-Host "LB TEST PASSED"
