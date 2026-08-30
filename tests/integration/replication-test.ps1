# Replication integration test for tiny-aws.
# Requires two object-store instances:
#   Node 1: OBJECT_STORE_ADDR=127.0.0.1:7001 STORAGE_ROOT=data1 METADATA_DB=metadata1.db REPLICATION_FACTOR=2
#   Node 2: OBJECT_STORE_ADDR=127.0.0.1:7002 STORAGE_ROOT=data2 METADATA_DB=metadata2.db
# Run from repo root: .\tests\integration\replication-test.ps1

$ErrorActionPreference = "Stop"

$node1 = "http://127.0.0.1:7001"
$node2 = "http://127.0.0.1:7002"
$key   = "repl-test-$(Get-Date -Format 'HHmmss')"

$curlAuth = @()
if ($env:TINYAWS_API_KEY) { $curlAuth = @("-H", "Authorization: Bearer $env:TINYAWS_API_KEY") }

Write-Host "replication integration test"
Write-Host ""

Write-Host "[1/5] Node health"
$h1 = curl.exe -sf @curlAuth "$node1/health"
if ($h1 -notmatch "healthy") { throw "node1 not healthy" }
Write-Host "  node1 ok"
$h2 = curl.exe -sf @curlAuth "$node2/health"
if ($h2 -notmatch "healthy") { throw "node2 not healthy" }
Write-Host "  node2 ok"

Write-Host "[2/5] PUT object to node 1"
curl.exe -sf @curlAuth -X PUT "$node1/objects/$key" -d "replication-data" | Out-Null
if ($LASTEXITCODE -ne 0) { throw "PUT to node1 failed" }
Write-Host "  PUT ok"

Write-Host "[3/5] GET object from node 1 (primary)"
$body1 = curl.exe -sf @curlAuth "$node1/objects/$key"
if ($body1 -ne "replication-data") { throw "GET from node1 mismatch: $body1" }
Write-Host "  GET node1 ok"

Write-Host "[4/5] GET object from node 2 (replica — retry up to 10s)"
$body2 = $null
for ($i = 0; $i -lt 5; $i++) {
    Start-Sleep -Seconds 2
    $body2 = curl.exe -sf @curlAuth "$node2/objects/$key" 2>$null
    if ($body2 -eq "replication-data") { break }
}
if ($body2 -ne "replication-data") { throw "GET from node2 mismatch after retries: $body2" }
Write-Host "  GET node2 ok"

Write-Host "[5/5] DELETE propagated to node 2"
curl.exe -sf @curlAuth -X DELETE "$node1/objects/$key" | Out-Null
if ($LASTEXITCODE -ne 0) { throw "DELETE on node1 failed" }

$deleted = $false
for ($i = 0; $i -lt 5; $i++) {
    Start-Sleep -Seconds 2
    $code = curl.exe -s -o NUL -w "%{http_code}" @curlAuth "$node2/objects/$key" 2>$null
    if ($code -eq "404") { $deleted = $true; break }
}
if (-not $deleted) { throw "DELETE did not propagate to node2 within 10s" }
Write-Host "  DELETE propagation ok"

Write-Host ""
Write-Host "REPLICATION TEST PASSED"
