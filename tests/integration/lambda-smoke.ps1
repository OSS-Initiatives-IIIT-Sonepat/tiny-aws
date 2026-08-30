# Lambda invoke smoke test.
# Requires: lambda service, scheduler, ec2-agent, object-store all running.
# Run from repo root: .\tests\integration\lambda-smoke.ps1

$ErrorActionPreference = "Stop"
$lambda = "http://127.0.0.1:9007"

Write-Host "lambda smoke test"
Write-Host ""

Write-Host "[1/3] Lambda health"
$h = Invoke-RestMethod "$lambda/health"
if ($h.status -ne "healthy") { throw "lambda not healthy" }
Write-Host "  healthy"

Write-Host "[2/3] Create function"
$fn = Invoke-RestMethod -Uri "$lambda/functions" -Method Post -ContentType "application/json" `
  -Body '{"name":"smoke-fn","runtime":"python3","handler":"handler.handler","bucket":"lambdas","key":"smoke-fn.zip"}'
if ($fn.name -ne "smoke-fn") { throw "create failed" }
Write-Host "  created smoke-fn"

Write-Host "[3/3] List functions"
$list = Invoke-RestMethod "$lambda/functions"
$found = $list | Where-Object { $_.name -eq "smoke-fn" }
if (-not $found) { throw "function not in list" }
Write-Host "  found in list"

Write-Host ""
Write-Host "LAMBDA SMOKE TEST PASSED"
