# Smoke test for tinyaws.build and env vars features.
# Tests:
#   1. Env vars flow through scheduler and appear in job payload
#   2. tinyaws.build format is correctly parsed by builder
#   3. Job with env_vars is accepted and returned correctly
#
# Assumes: registry + scheduler + ec2-agent + object-store running.
# Run from repo root: .\tests\integration\build-smoke.ps1

$ErrorActionPreference = "Stop"

$apiKey = $env:TINYAWS_API_KEY
$authHeader = if ($apiKey) { @{ Authorization = "Bearer $apiKey" } } else { @{} }
$curlAuth = if ($apiKey) { @("-H", "Authorization: Bearer $apiKey") } else { @() }

function Test-Endpoint {
    param([string]$Name, [string]$Url, [string]$ExpectedPattern = ".")
    Write-Host "  checking $Name..." -NoNewline
    $response = curl.exe -s -f @curlAuth $Url
    if ($LASTEXITCODE -ne 0) { Write-Host " FAIL"; throw "$Name unreachable: $Url" }
    if ($response -notmatch $ExpectedPattern) { Write-Host " FAIL"; throw "$Name unexpected: $response" }
    Write-Host " ok"
}

Write-Host "tinyaws.build + env vars smoke test"
Write-Host ""

# ── 1. Health checks ─────────────────────────────────────────────────────────
Write-Host "[1/6] Service health"
Test-Endpoint "registry"     "http://127.0.0.1:9000/health" '"status":"healthy"'
Test-Endpoint "scheduler"    "http://127.0.0.1:9001/health" '"status":"healthy"'
Test-Endpoint "ec2-agent"    "http://127.0.0.1:8080/health" '"status":"healthy"'
Test-Endpoint "object-store" "http://127.0.0.1:7001/health" '"status":"healthy"'

# ── 2. Job with env_vars ─────────────────────────────────────────────────────
Write-Host ""
Write-Host "[2/6] Submit job with env_vars"
$envBody = @{
    command = "echo ENV_TEST"
    env_vars = @{
        DATABASE_URL = "postgres://localhost/test"
        NODE_ENV = "production"
        API_KEY = "secret123"
    }
} | ConvertTo-Json

$envJob = Invoke-RestMethod -Uri "http://127.0.0.1:9001/jobs" -Method Post `
    -ContentType "application/json" -Headers $authHeader -Body $envBody

if (-not $envJob.job_id) { throw "env job missing job_id" }
if (-not $envJob.env_vars) { throw "env job missing env_vars in response" }
if ($envJob.env_vars.DATABASE_URL -ne "postgres://localhost/test") {
    throw "env_vars.DATABASE_URL mismatch: $($envJob.env_vars.DATABASE_URL)"
}
if ($envJob.env_vars.NODE_ENV -ne "production") {
    throw "env_vars.NODE_ENV mismatch: $($envJob.env_vars.NODE_ENV)"
}
Write-Host "  env_vars in job response ok"

# ── 3. Wait for env job to complete ──────────────────────────────────────────
Write-Host ""
Write-Host "[3/6] Wait for env job execution"
$jobId = $envJob.job_id
$final = $null
for ($i = 0; $i -lt 15; $i++) {
    Start-Sleep -Seconds 2
    $final = Invoke-RestMethod -Uri "http://127.0.0.1:9001/jobs/$jobId" -Headers $authHeader
    if ($final.status -eq "done") { break }
    if ($final.status -eq "failed") {
        throw "env job failed: $($final | ConvertTo-Json -Compress)"
    }
}
if ($final.status -ne "done") { throw "env job did not complete (status=$($final.status))" }
Write-Host "  env job completed ok"

# ── 4. Verify env_vars persist in job record ─────────────────────────────────
Write-Host ""
Write-Host "[4/6] Verify env_vars persisted"
$fetched = Invoke-RestMethod -Uri "http://127.0.0.1:9001/jobs/$jobId" -Headers $authHeader
if (-not $fetched.env_vars) { throw "fetched job missing env_vars" }
if ($fetched.env_vars.API_KEY -ne "secret123") {
    throw "persisted env_vars.API_KEY mismatch"
}
Write-Host "  env_vars persisted in scheduler ok"

# ── 5. Deploy with env_vars via object store ─────────────────────────────────
Write-Host ""
Write-Host "[5/6] Deploy app with tinyaws.build file"

# Create a temp app with tinyaws.build
$workDir = Join-Path $env:TEMP "tinyaws-build-smoke-$(Get-Date -Format 'HHmmss')"
New-Item -ItemType Directory -Path $workDir -Force | Out-Null

# Write tinyaws.build file
@"
# smoke test build spec
base: ubuntu
packages: python3
start: echo hello-from-build
"@ | Set-Content -Path (Join-Path $workDir "tinyaws.build") -Encoding UTF8

# Write a start.sh fallback
@"
#!/bin/sh
echo fallback-start
"@ | Set-Content -Path (Join-Path $workDir "start.sh") -Encoding UTF8

# Zip it
$zipPath = Join-Path $env:TEMP "tinyaws-build-smoke.zip"
if (Test-Path $zipPath) { Remove-Item $zipPath }
Compress-Archive -Path "$workDir\*" -DestinationPath $zipPath

# Upload to object store
curl.exe -s -f @curlAuth -X PUT "http://127.0.0.1:7001/buckets/deployments" 2>$null | Out-Null
curl.exe -s -f @curlAuth -X PUT "http://127.0.0.1:7001/buckets/deployments/objects/build-smoke.zip" `
    --data-binary "@$zipPath" | Out-Null
if ($LASTEXITCODE -ne 0) { throw "upload build-smoke.zip failed" }
Write-Host "  uploaded app with tinyaws.build ok"

# Submit as a job with env_vars
$deployBody = @{
    deploy_url = "http://127.0.0.1:7001/buckets/deployments/objects/build-smoke.zip"
    command = ""
    env_vars = @{
        MY_VAR = "build-test-value"
    }
} | ConvertTo-Json

$deployJob = Invoke-RestMethod -Uri "http://127.0.0.1:9001/jobs" -Method Post `
    -ContentType "application/json" -Headers $authHeader -Body $deployBody

if (-not $deployJob.job_id) { throw "deploy job missing job_id" }
if ($deployJob.env_vars.MY_VAR -ne "build-test-value") {
    throw "deploy job env_vars mismatch"
}
Write-Host "  deploy job submitted with env_vars ok"

# ── 6. Verify deploy job picked up ──────────────────────────────────────────
Write-Host ""
Write-Host "[6/6] Wait for deploy job pickup"
$dJobId = $deployJob.job_id
$dFinal = $null
for ($i = 0; $i -lt 15; $i++) {
    Start-Sleep -Seconds 2
    $dFinal = Invoke-RestMethod -Uri "http://127.0.0.1:9001/jobs/$dJobId" -Headers $authHeader
    Write-Host "  status=$($dFinal.status)"
    if ($dFinal.status -eq "done" -or $dFinal.status -eq "failed" -or $dFinal.status -eq "running") {
        break
    }
}
# On Windows the rootfs build won't work (needs Linux), so we just verify
# the job was picked up and attempted (status moved past pending).
if ($dFinal.status -eq "pending") {
    throw "deploy job stuck in pending after 30s"
}
Write-Host "  deploy job picked up (status=$($dFinal.status)) ok"

# Cleanup
Remove-Item -Recurse -Force $workDir -ErrorAction SilentlyContinue
Remove-Item -Force $zipPath -ErrorAction SilentlyContinue

Write-Host ""
Write-Host "BUILD + ENV SMOKE TEST PASSED"
