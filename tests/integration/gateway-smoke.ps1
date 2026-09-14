# API Gateway integration smoke test.
# Verifies that the gateway correctly routes /v1/* to backend services.
# Requires: registry + scheduler + object-store + api-gateway running.
# Run from repo root: .\tests\integration\gateway-smoke.ps1

$ErrorActionPreference = "Stop"

$gw       = if ($env:TINYAWS_API_URL) { $env:TINYAWS_API_URL } else { "http://127.0.0.1:8000" }
$registry = if ($env:REGISTRY_URL)    { $env:REGISTRY_URL }    else { "http://127.0.0.1:9000" }

$curlAuth = @()
if ($env:TINYAWS_API_KEY) {
    $curlAuth = @("-H", "Authorization: Bearer $($env:TINYAWS_API_KEY)")
}
$authHeader = if ($env:TINYAWS_API_KEY) { @{ Authorization = "Bearer $($env:TINYAWS_API_KEY)" } } else { @{} }

function Assert-Curl {
    param([string]$Name, [string]$Url, [string]$Pattern)
    $resp = curl.exe -s -f @curlAuth $Url
    if ($LASTEXITCODE -ne 0) { throw "FAIL: $Name unreachable" }
    if ($resp -notmatch $Pattern) { throw "FAIL: $Name unexpected: $resp" }
    Write-Host "  $Name ok"
}

Write-Host "API gateway smoke test"
Write-Host ""

Write-Host "[1/6] Gateway health endpoints"
Assert-Curl "/v1/health/registry"  "$gw/v1/health/registry"  "healthy"
Assert-Curl "/v1/health/scheduler" "$gw/v1/health/scheduler" "healthy"
Assert-Curl "/v1/health/store"     "$gw/v1/health/store"     "healthy"

Write-Host ""
Write-Host "[2/6] /v1/nodes routes to registry"
Assert-Curl "/v1/nodes" "$gw/v1/nodes" "\{"

Write-Host ""
Write-Host "[3/6] /v1/jobs routes to scheduler"
Assert-Curl "/v1/jobs" "$gw/v1/jobs" "\["

Write-Host ""
Write-Host "[4/6] /v1/objects routes to object-store (PUT then GET)"
$gwKey = "gw-smoke-$(Get-Date -Format 'HHmmss')"
curl.exe -s -f @curlAuth -X PUT "$gw/v1/objects/$gwKey" -d "gateway-test" | Out-Null
if ($LASTEXITCODE -ne 0) { throw "FAIL: /v1/objects PUT" }
$gbody = curl.exe -s -f @curlAuth "$gw/v1/objects/$gwKey"
if ($gbody -ne "gateway-test") { throw "FAIL: /v1/objects GET mismatch: $gbody" }
Write-Host "  /v1/objects PUT+GET ok"

Write-Host ""
Write-Host "[5/6] /v1/instances routes to registry"
$inst = Invoke-RestMethod -Uri "$gw/v1/instances" -Method Post `
    -ContentType "application/json" -Headers $authHeader `
    -Body '{"instance_type":"small"}'
if (-not $inst.id) { throw "FAIL: /v1/instances POST returned no id" }
Write-Host "  /v1/instances POST -> $($inst.id) ok"

Write-Host ""
Write-Host "[6/6] Compare direct vs gateway response"
$direct = curl.exe -s -f @curlAuth "$registry/instances/$($inst.id)"
$viaGw  = curl.exe -s -f @curlAuth "$gw/v1/instances/$($inst.id)"
if ($direct -ne $viaGw) { throw "FAIL: direct vs gateway response mismatch" }
Write-Host "  gateway response matches direct ok"

Write-Host ""
Write-Host "GATEWAY SMOKE TEST PASSED"
