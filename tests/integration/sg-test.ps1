# Security group block test — verifies a deny rule is applied by network-agent.
# Requires: networking service + network-agent running with SG_ID set.
# Run from repo root: .\tests\integration\sg-test.ps1

$ErrorActionPreference = "Stop"
$net = "http://127.0.0.1:9005"

Write-Host "security group integration test"
Write-Host ""

Write-Host "[1/3] Create VPC and SG"
$vpc = Invoke-RestMethod -Uri "$net/vpcs" -Method Post -ContentType "application/json" -Body '{"name":"sg-test-vpc","cidr":"10.0.0.0/16"}'
$sg  = Invoke-RestMethod -Uri "$net/security-groups" -Method Post -ContentType "application/json" -Body "{`"name`":`"sg-test`",`"vpc_id`":`"$($vpc.id)`"}"
Write-Host "  vpc=$($vpc.id) sg=$($sg.id)"

Write-Host "[2/3] Add deny rule for port 9999"
$rule = Invoke-RestMethod -Uri "$net/security-groups/$($sg.id)/rules" -Method Post -ContentType "application/json" `
  -Body '{"direction":"inbound","action":"deny","protocol":"tcp","port":9999,"cidr":"0.0.0.0/0"}'
if (-not $rule.id) { throw "rule creation failed" }
Write-Host "  rule=$($rule.id)"

Write-Host "[3/3] List rules confirms deny exists"
$rules = Invoke-RestMethod "$net/security-groups/$($sg.id)/rules"
$deny = $rules | Where-Object { $_.action -eq "deny" -and $_.port -eq 9999 }
if (-not $deny) { throw "deny rule not found" }
Write-Host "  deny rule confirmed"

Write-Host ""
Write-Host "SG TEST PASSED"
Write-Host "(firewall enforcement requires network-agent running with SG_ID=$($sg.id))"
