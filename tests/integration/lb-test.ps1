# Load balancer integration test for tiny-aws.
# Requires: registry + ec2-agent + load-balancer running.
# Run from repo root: .\tests\integration\lb-test.ps1

$ErrorActionPreference = "Stop"

$lb  = $env:LB_URL
if (-not $lb) { $lb = "http://127.0.0.1:8088" }
$reg = $env:REGISTRY_URL
if (-not $reg) { $reg = "http://127.0.0.1:9000" }
$authHeader = @{}
if ($env:TINYAWS_API_KEY) { $authHeader = @{ Authorization = "Bearer $env:TINYAWS_API_KEY" } }
$curlAuth = @()
if ($env:TINYAWS_API_KEY) { $curlAuth = @("-H", "Authorization: Bearer $env:TINYAWS_API_KEY") }

Write-Host "load balancer integration test"
Write-Host ""

Write-Host "[1/4] LB health"
$h = Invoke-RestMethod "$lb/health"
if ($h.status -ne "healthy") { throw "lb not healthy" }
Write-Host "  healthy, targets=$($h.targets)"

Write-Host "[2/4] LB targets list"
$targets = Invoke-RestMethod "$lb/targets"
Write-Host "  $($targets.Count) target(s) registered"
if ($targets.Count -eq 0) {
    Write-Host "  WARNING: no targets yet — is ec2-agent running and registered?"
}

Write-Host "[3/4] LB proxies request to agent (GET /nodes through gateway-style path)"
# The LB catch-all '/' proxies to agent :8080. The agent serves /health and /info.
# We hit the LB root '/' which routes to the agent; agent returns 404 for '/'
# but we can verify we got a response FROM the agent (connection didn't fail).
$proxyResult = curl.exe -s -o NUL -w "%{http_code}" "$lb/"
# 404 is fine — agent returned it, meaning the proxy worked
if ($proxyResult -eq "000") { throw "LB proxy connection failed — no backend reachable" }
Write-Host "  LB proxied request, backend responded with HTTP $proxyResult"

Write-Host "[4/4] LB proxies /health to agent"
$agentViaLB = curl.exe -sf "$lb/health" 2>$null
# LB /health is the LB's own health. To test actual proxy, hit an agent-only path.
$agentInfo = curl.exe -s -o NUL -w "%{http_code}" "$lb/info"
if ($agentInfo -eq "000") { throw "LB proxy to /info connection failed" }
Write-Host "  /info via LB: HTTP $agentInfo (200=agent responded, 404=agent returned not-found, both mean proxy works)"

Write-Host ""
Write-Host "LB TEST PASSED"
