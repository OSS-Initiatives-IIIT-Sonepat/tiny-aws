# Networking (VPC/subnet/route/SG) integration test for tiny-aws.
# Requires: networking service running (NETWORKING_ADDR=:9005 go run . in control-plane/networking/vpc)
# Run from repo root: .\tests\integration\sg-test.ps1

$ErrorActionPreference = "Stop"

$net = $env:NETWORKING_URL
if (-not $net) { $net = "http://127.0.0.1:9005" }
$reg = $env:REGISTRY_URL
if (-not $reg) { $reg = "http://127.0.0.1:9000" }
$authHeader = @{}
if ($env:TINYAWS_API_KEY) { $authHeader = @{ Authorization = "Bearer $env:TINYAWS_API_KEY" } }

Write-Host "networking integration test (VPC / subnet / route / SG)"
Write-Host ""

Write-Host "[1/9] Networking health"
$h = Invoke-RestMethod "$net/health"
if ($h.status -ne "healthy") { throw "networking service not healthy" }
Write-Host "  healthy"

Write-Host "[2/9] Create VPC"
$vpc = Invoke-RestMethod -Uri "$net/vpcs" -Method Post -ContentType "application/json" `
  -Body '{"name":"test-vpc","cidr":"10.10.0.0/16"}'
if (-not $vpc.id) { throw "VPC create failed" }
Write-Host "  vpc=$($vpc.id) cidr=$($vpc.cidr)"

Write-Host "[3/9] List VPCs — appears in list"
$vpcs = Invoke-RestMethod "$net/vpcs"
if (-not ($vpcs | Where-Object { $_.id -eq $vpc.id })) { throw "VPC not in list" }
Write-Host "  VPC in list ok"

Write-Host "[4/9] Get VPC by ID"
$vpcGet = Invoke-RestMethod "$net/vpcs/$($vpc.id)"
if ($vpcGet.id -ne $vpc.id) { throw "GET /vpcs/{id} mismatch" }
Write-Host "  GET VPC ok"

Write-Host "[5/9] Create subnet in VPC"
$subnet = Invoke-RestMethod -Uri "$net/subnets" -Method Post -ContentType "application/json" `
  -Body "{`"vpc_id`":`"$($vpc.id)`",`"name`":`"test-subnet`",`"cidr`":`"10.10.1.0/24`"}"
if (-not $subnet.id) { throw "subnet create failed" }
Write-Host "  subnet=$($subnet.id)"

Write-Host "[6/9] List subnets filtered by VPC"
$subnets = Invoke-RestMethod "$net/subnets?vpc_id=$($vpc.id)"
if (-not ($subnets | Where-Object { $_.id -eq $subnet.id })) { throw "subnet not in filtered list" }
Write-Host "  subnet in list ok"

Write-Host "[7/9] Create route table for subnet"
$rt = Invoke-RestMethod -Uri "$net/route-tables" -Method Post -ContentType "application/json" `
  -Body "{`"subnet_id`":`"$($subnet.id)`",`"destination`":`"0.0.0.0/0`",`"target`":`"igw`"}"
if (-not $rt.id) { throw "route table create failed" }
Write-Host "  route=$($rt.id)"

Write-Host "[8/9] Create security group + allow and deny rules"
$sg = Invoke-RestMethod -Uri "$net/security-groups" -Method Post -ContentType "application/json" `
  -Body "{`"name`":`"test-sg`",`"vpc_id`":`"$($vpc.id)`"}"
if (-not $sg.id) { throw "SG create failed" }
Write-Host "  sg=$($sg.id)"

# allow rule
$allowRule = Invoke-RestMethod -Uri "$net/security-groups/$($sg.id)/rules" -Method Post `
  -ContentType "application/json" `
  -Body '{"direction":"inbound","action":"allow","protocol":"tcp","port":80,"cidr":"0.0.0.0/0"}'
if (-not $allowRule.id) { throw "allow rule create failed" }

# deny rule
$denyRule = Invoke-RestMethod -Uri "$net/security-groups/$($sg.id)/rules" -Method Post `
  -ContentType "application/json" `
  -Body '{"direction":"inbound","action":"deny","protocol":"tcp","port":22,"cidr":"0.0.0.0/0"}'
if (-not $denyRule.id) { throw "deny rule create failed" }

$rules = Invoke-RestMethod "$net/security-groups/$($sg.id)/rules"
$hasAllow = $rules | Where-Object { $_.action -eq "allow" -and $_.port -eq 80 }
$hasDeny  = $rules | Where-Object { $_.action -eq "deny"  -and $_.port -eq 22 }
if (-not $hasAllow) { throw "allow rule not in rule list" }
if (-not $hasDeny)  { throw "deny rule not in rule list" }
Write-Host "  allow+deny rules ok"

Write-Host "[9/9] Assign instance to subnet and read back"
# launch an instance to assign
$inst = Invoke-RestMethod -Uri "$reg/instances" -Method Post -Headers $authHeader `
  -ContentType "application/json" -Body '{"instance_type":"small"}'
if (-not $inst.id) { throw "instance launch failed" }

Invoke-RestMethod -Uri "$net/instances/$($inst.id)/subnet" -Method Put `
  -ContentType "application/json" `
  -Body "{`"subnet_id`":`"$($subnet.id)`"}" | Out-Null

$assignment = Invoke-RestMethod "$net/instances/$($inst.id)/subnet"
if ($assignment.subnet_id -ne $subnet.id) { throw "subnet assignment mismatch" }
Write-Host "  instance→subnet assignment ok"

Write-Host ""
Write-Host "NETWORKING TEST PASSED"
Write-Host "(firewall enforcement requires network-agent running with SG_ID=$($sg.id))"
