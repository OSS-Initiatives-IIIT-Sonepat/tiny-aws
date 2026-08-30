# Replication integration test for tiny-aws.
# Requires two object-store instances:
#   Node 1: OBJECT_STORE_ADDR=127.0.0.1:7001 STORAGE_ROOT=data1 METADATA_DB=metadata1.db
#   Node 2: OBJECT_STORE_ADDR=127.0.0.1:7002 STORAGE_ROOT=data2 METADATA_DB=metadata2.db REPLICATION_FACTOR=2
# Both must be registered with the same registry.
# Run from repo root: .\tests\integration\replication-test.ps1

$ErrorActionPreference = "Stop"

$apiKey = $env:TINYAWS_API_KEY
$curlAuth = if ($apiKey) { @("-H", "Authorization: Bearer $apiKey") } else { @() }

$node1 = "http://127.0.0.1:7001"
$node2 = "http://127.0.0.1:7002"
$key = "repl-test-$(Get-Date -Format 'HHmmss')"

Write-Host "replication integration test"
Write-Host ""

Write-Host "[1/3] PUT object to node 1"
curl.exe -s -f @curlAuth -X PUT "$node1/objects/$key" -d "replication-data" | Out-Null
if ($LASTEXITCODE -ne 0) { throw "PUT to node1 failed" }
Write-Host "  PUT ok"

Write-Host "[2/3] GET object from node 1 (primary)"
$body1 = curl.exe -s -f @curlAuth "$node1/objects/$key"
if ($body1 -ne "replication-data") { throw "GET from node1 mismatch: $body1" }
Write-Host "  GET node1 ok"

Write-Host "[3/3] GET object from node 2 (replica)"
Start-Sleep -Seconds 1
$body2 = curl.exe -s -f @curlAuth "$node2/objects/$key"
if ($body2 -ne "replication-data") { throw "GET from node2 mismatch: $body2" }
Write-Host "  GET node2 ok"

Write-Host ""
Write-Host "REPLICATION TEST PASSED"
