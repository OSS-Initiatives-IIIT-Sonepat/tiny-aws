# Chaos test: kill agent mid-job.
# Submits a long-running job, kills the agent process, verifies scheduler marks it failed.
# Run from repo root: .\tests\chaos\agent-kill.ps1

$ErrorActionPreference = "Stop"
$sch = "http://127.0.0.1:9001"
$reg = "http://127.0.0.1:9000"

Write-Host "=== chaos: kill agent mid-job ==="
Write-Host ""

# submit a 30s sleep job
Write-Host "[1] Submit long-running job (30s sleep)"
$job = Invoke-RestMethod -Uri "$sch/jobs" -Method Post -ContentType "application/json" -Body '{"command":"ping -n 30 127.0.0.1"}'
$jobID = $job.job_id
Write-Host "  job=$jobID"

# wait for it to start running
Write-Host "[2] Wait for job to reach running state..."
for ($i = 0; $i -lt 10; $i++) {
    Start-Sleep -Seconds 2
    $status = (Invoke-RestMethod "$sch/jobs/$jobID").status
    if ($status -eq "running") { break }
}
if ($status -ne "running") { throw "job never reached running (status=$status)" }
Write-Host "  running"

# kill the agent
Write-Host "[3] Killing ec2-agent process..."
Get-Process -Name "ec2-agent" -ErrorAction SilentlyContinue | Stop-Process -Force
Write-Host "  killed"

# wait for scheduler timeout (60s) to mark job failed
Write-Host "[4] Waiting for scheduler to timeout job (up to 75s)..."
$finalStatus = $null
for ($i = 0; $i -lt 40; $i++) {
    Start-Sleep -Seconds 2
    $finalStatus = (Invoke-RestMethod "$sch/jobs/$jobID").status
    if ($finalStatus -eq "failed") { break }
}
if ($finalStatus -ne "failed") { throw "job not marked failed after agent kill (status=$finalStatus)" }
Write-Host "  job marked failed by scheduler timeout"

Write-Host ""
Write-Host "CHAOS TEST PASSED"
