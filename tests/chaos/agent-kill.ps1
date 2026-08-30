# Chaos test: kill agent mid-job.
# Submits a long-running job, kills the agent process, verifies scheduler marks it failed.
# Run from repo root: .\tests\chaos\agent-kill.ps1
#
# NOTE: sets JOB_TIMEOUT_SECS=60 in environment before the test — requires the
#       scheduler to have been started with this env var, or restarted here.
#       The default timeout is 3600s which would make this test wait 1 hour.

$ErrorActionPreference = "Stop"

$sch = $env:SCHEDULER_URL
if (-not $sch) { $sch = "http://127.0.0.1:9001" }
$authHeader = @{}
if ($env:TINYAWS_API_KEY) { $authHeader = @{ Authorization = "Bearer $env:TINYAWS_API_KEY" } }

Write-Host "=== chaos: kill agent mid-job ==="
Write-Host "NOTE: scheduler must be running with JOB_TIMEOUT_SECS=60"
Write-Host ""

Write-Host "[1/4] Submit long-running job (60s sleep)"
$job = Invoke-RestMethod -Uri "$sch/jobs" -Method Post -ContentType "application/json" `
  -Headers $authHeader -Body '{"command":"sleep 60"}'
$jobID = $job.job_id
if (-not $jobID) { throw "job submission failed" }
Write-Host "  job=$jobID"

Write-Host "[2/4] Wait for job to reach running state..."
$running = $false
for ($i = 0; $i -lt 15; $i++) {
    Start-Sleep -Seconds 2
    $s = (Invoke-RestMethod -Uri "$sch/jobs/$jobID" -Headers $authHeader).status
    if ($s -eq "running") { $running = $true; break }
}
if (-not $running) { throw "job never reached running" }
Write-Host "  running"

Write-Host "[3/4] Killing ec2-agent process..."
$proc = Get-Process -Name "ec2-agent" -ErrorAction SilentlyContinue
if (-not $proc) {
    # try the .exe variant
    $proc = Get-Process -Name "ec2-agent.exe" -ErrorAction SilentlyContinue
}
if (-not $proc) {
    throw "ec2-agent process not found — is it running? (process name must be 'ec2-agent' or 'ec2-agent.exe')"
}
$proc | Stop-Process -Force
Write-Host "  killed pid=$($proc.Id)"

Write-Host "[4/4] Waiting for scheduler to timeout job (JOB_TIMEOUT_SECS=60, checking up to 90s)..."
$finalStatus = $null
for ($i = 0; $i -lt 45; $i++) {
    Start-Sleep -Seconds 2
    $finalStatus = (Invoke-RestMethod -Uri "$sch/jobs/$jobID" -Headers $authHeader).status
    if ($finalStatus -eq "failed") { break }
}
if ($finalStatus -ne "failed") {
    throw "job not marked failed after agent kill (status=$finalStatus). Is JOB_TIMEOUT_SECS=60 set on scheduler?"
}
Write-Host "  job marked failed by scheduler timeout"

Write-Host ""
Write-Host "CHAOS TEST PASSED"
Write-Host "Remember to restart the ec2-agent before running other tests."
