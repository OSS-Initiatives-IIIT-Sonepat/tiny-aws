# Service deploy smoke test for tiny-aws on Windows.
# Tests that a service job type is accepted, spawned, and registered.
# Requires: registry + scheduler + ec2-agent + object-store running.
# Run from repo root: .\tests\integration\service-smoke.ps1

$ErrorActionPreference = "Stop"

$REGISTRY  = if ($env:REGISTRY_URL)     { $env:REGISTRY_URL }     else { "http://127.0.0.1:9000" }
$SCHEDULER = if ($env:SCHEDULER_URL)    { $env:SCHEDULER_URL }    else { "http://127.0.0.1:9001" }
$STORE     = if ($env:OBJECT_STORE_URL) { $env:OBJECT_STORE_URL } else { "http://127.0.0.1:7001" }

$headers = @{}
if ($env:TINYAWS_API_KEY) {
    $headers["Authorization"] = "Bearer $($env:TINYAWS_API_KEY)"
}

function ok($msg)   { Write-Host "  $msg ok" }
function fail($msg) { Write-Host "FAIL: $msg"; exit 1 }

Write-Host "service deploy smoke test"
Write-Host ""

# [1/6] Check health
Write-Host "[1/6] Check health"
try {
    $r = Invoke-RestMethod -Uri "$REGISTRY/health" -Headers $headers
    if ($r.status -ne "healthy") { fail "registry" }
    ok "registry"
} catch { fail "registry: $_" }

try {
    $r = Invoke-RestMethod -Uri "$STORE/health"
    if ($r.status -ne "healthy") { fail "object-store" }
    ok "object-store"
} catch { fail "object-store: $_" }

try {
    $r = Invoke-RestMethod -Uri "$SCHEDULER/health" -Headers $headers
    if ($r.status -ne "healthy") { fail "scheduler" }
    ok "scheduler"
} catch { fail "scheduler: $_" }

# [2/6] Create test app with start.ps1
Write-Host "[2/6] Create test app with start.ps1"
$workDir = Join-Path $env:TEMP "tinyaws-svc-smoke-$(Get-Random)"
New-Item -ItemType Directory -Path $workDir -Force | Out-Null

$startScript = @'
Write-Host "tiny-aws service started"
$listener = [System.Net.HttpListener]::new()
$listener.Prefixes.Add("http://+:19999/")
$listener.Start()
while ($true) {
    $ctx = $listener.GetContext()
    $resp = $ctx.Response
    $body = [System.Text.Encoding]::UTF8.GetBytes("ok")
    $resp.ContentLength64 = $body.Length
    $resp.OutputStream.Write($body, 0, $body.Length)
    $resp.Close()
}
'@
Set-Content -Path (Join-Path $workDir "start.ps1") -Value $startScript
ok "app created at $workDir"

# [3/6] Upload app zip
Write-Host "[3/6] Upload app zip"
$zipPath = Join-Path $workDir "app.zip"
Compress-Archive -Path (Join-Path $workDir "start.ps1") -DestinationPath $zipPath -Force

try { Invoke-RestMethod -Method PUT -Uri "$STORE/buckets/deployments" -Headers $headers 2>$null } catch {}

$zipBytes = [System.IO.File]::ReadAllBytes($zipPath)
try {
    Invoke-RestMethod -Method PUT -Uri "$STORE/buckets/deployments/objects/smoke-svc-test.zip" `
        -Headers $headers -Body $zipBytes -ContentType "application/octet-stream" | Out-Null
    ok "uploaded"
} catch { fail "upload zip: $_" }

# [4/6] Submit service job
Write-Host "[4/6] Submit service job"
$deployUrl = "$STORE/buckets/deployments/objects/smoke-svc-test.zip"
$body = @{
    deploy_url = $deployUrl
    command    = ""
    job_type   = "service"
    port       = 19999
} | ConvertTo-Json

try {
    $resp = Invoke-RestMethod -Method POST -Uri "$SCHEDULER/jobs" -Headers $headers `
        -Body $body -ContentType "application/json"
    $jobId = $resp.job_id
    if (-not $jobId) { fail "job_id missing from response" }
    ok "job $jobId submitted"
} catch { fail "submit job: $_" }

# [5/6] Wait for job to reach running state
Write-Host "[5/6] Wait for job to reach running state"
$status = ""
for ($i = 1; $i -le 15; $i++) {
    Start-Sleep -Seconds 2
    try {
        $j = Invoke-RestMethod -Uri "$SCHEDULER/jobs/$jobId" -Headers $headers
        $status = $j.status
        Write-Host "  status=$status"
        if ($status -eq "running") { break }
        if ($status -eq "failed") {
            fail "job failed: $($j | ConvertTo-Json -Compress)"
        }
    } catch { Write-Host "  poll error: $_" }
}
if ($status -ne "running") { fail "job never reached running (last status=$status)" }
ok "service job running"

# [6/6] Verify service registered in registry
Write-Host "[6/6] Verify service registered in registry"
$svcId = ""
for ($i = 1; $i -le 5; $i++) {
    Start-Sleep -Seconds 3
    try {
        $svcs = Invoke-RestMethod -Uri "$REGISTRY/services" -Headers $headers
        foreach ($s in $svcs) {
            if ($s.id -match "^svc-") {
                $svcId = $s.id
                break
            }
        }
        if ($svcId) { break }
    } catch {}
}
if (-not $svcId) { fail "service not registered in registry after 15s" }
ok "service $svcId registered"

# Cleanup
Remove-Item -Recurse -Force $workDir -ErrorAction SilentlyContinue

Write-Host ""
Write-Host "SERVICE SMOKE TEST PASSED"
