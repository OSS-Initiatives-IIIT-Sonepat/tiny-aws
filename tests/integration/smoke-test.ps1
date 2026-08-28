# Integration smoke test for tiny-aws.
# Assumes the full stack is already running (see scripts/run-local.ps1).
# Run from repo root: .\tests\integration\smoke-test.ps1

$ErrorActionPreference = "Stop"

function Test-Endpoint {
    param(
        [string]$Name,
        [string]$Url,
        [string]$ExpectedPattern = "."
    )

    Write-Host "  checking $Name..." -NoNewline
    $response = curl.exe -s -f $Url
    if ($LASTEXITCODE -ne 0) {
        Write-Host " FAIL"
        throw "$Name unreachable: $Url"
    }
    if ($response -notmatch $ExpectedPattern) {
        Write-Host " FAIL"
        throw "$Name unexpected response: $response"
    }
    Write-Host " ok"
}

Write-Host "tiny-aws integration smoke test"
Write-Host ""

Write-Host "[1/8] Service health"
Test-Endpoint "registry"      "http://127.0.0.1:9000/health"   '"status":"healthy"'
Test-Endpoint "ec2-agent"     "http://127.0.0.1:8080/health"   '"status":"healthy"'
Test-Endpoint "scheduler"     "http://127.0.0.1:9001/health"   '"status":"healthy"'

Write-Host ""
Write-Host "[2/8] Registry nodes"
Test-Endpoint "nodes"         "http://127.0.0.1:9000/nodes"    "."
Test-Endpoint "compute nodes" "http://127.0.0.1:9000/nodes?role=compute" "."
Test-Endpoint "storage nodes" "http://127.0.0.1:9000/nodes?role=storage" "."

Write-Host ""
Write-Host "[3/8] Object store"
$objectKey = "smoke-test-$(Get-Date -Format 'HHmmss')"
curl.exe -s -f -X PUT "http://127.0.0.1:7001/objects/$objectKey" -d "hello integration" | Out-Null
if ($LASTEXITCODE -ne 0) { throw "object PUT failed" }
Write-Host "  put object ok"

$body = curl.exe -s -f "http://127.0.0.1:7001/objects/$objectKey"
if ($body -ne "hello integration") { throw "object GET mismatch: $body" }
Write-Host "  get object ok"

$meta = curl.exe -s -f "http://127.0.0.1:7001/objects/$objectKey/meta"
if ($meta -notmatch $objectKey) { throw "object meta missing key" }
Write-Host "  object meta ok"

Write-Host ""
Write-Host "[4/8] Scheduler"
Test-Endpoint "schedule"      "http://127.0.0.1:9001/schedule" '"node_id"'

Write-Host ""
Write-Host "[5/8] Job submission"
$jobResponse = Invoke-RestMethod -Uri "http://127.0.0.1:9001/jobs" `
    -Method Post `
    -ContentType "application/json" `
    -Body '{"command":"echo hello"}'
if (-not $jobResponse.job_id) { throw "job response missing job_id" }
if (-not $jobResponse.node_id) { throw "job response missing node_id" }
Write-Host "  submit job ok"

Write-Host ""
Write-Host "[6/8] Job execution"
$jobId = $jobResponse.job_id
$finalStatus = $null

for ($i = 0; $i -lt 15; $i++) {
    Start-Sleep -Seconds 2
    $finalStatus = Invoke-RestMethod -Uri "http://127.0.0.1:9001/jobs/$jobId"

    if ($finalStatus.status -eq "done") { break }
    if ($finalStatus.status -eq "failed") {
        throw "job failed: $($finalStatus | ConvertTo-Json -Compress)"
    }
}

if ($finalStatus.status -ne "done") {
    throw "job did not complete in time (status=$($finalStatus.status))"
}
Write-Host "  job completed ok"

Write-Host ""
Write-Host "[7/8] Buckets"
$bucket = "smoke-bucket-$(Get-Date -Format 'HHmmss')"
curl.exe -s -f -X PUT "http://127.0.0.1:7001/buckets/$bucket" | Out-Null
if ($LASTEXITCODE -ne 0) { throw "bucket create failed" }
Write-Host "  create bucket ok"

curl.exe -s -f -X PUT "http://127.0.0.1:7001/buckets/$bucket/objects/test.txt" -d "bucket hello" | Out-Null
if ($LASTEXITCODE -ne 0) { throw "bucket object PUT failed" }
Write-Host "  put bucket object ok"

$bucketBody = curl.exe -s -f "http://127.0.0.1:7001/buckets/$bucket/objects/test.txt"
if ($bucketBody -ne "bucket hello") { throw "bucket GET mismatch: $bucketBody" }
Write-Host "  get bucket object ok"

Write-Host ""
Write-Host "[8/8] Instances"
$inst = Invoke-RestMethod -Uri "http://127.0.0.1:9000/instances" -Method Post
if (-not $inst.id) { throw "instance launch failed" }
Write-Host "  launch instance ok"

Write-Host ""
Write-Host "ALL CHECKS PASSED"
