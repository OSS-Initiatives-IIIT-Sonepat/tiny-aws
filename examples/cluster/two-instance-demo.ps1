# Two-instance ping demo through VPC
# Run from repo root after starting the full stack.

$ErrorActionPreference = "Stop"
$net = "http://127.0.0.1:9005"
$reg = "http://127.0.0.1:9000"
$sch = "http://127.0.0.1:9001"

Write-Host "=== two-instance cluster demo ==="

# 1. Create VPC and subnet
$vpc    = Invoke-RestMethod -Uri "$net/vpcs"    -Method Post -ContentType "application/json" -Body '{"name":"demo-vpc","cidr":"10.0.0.0/16"}'
$subnet = Invoke-RestMethod -Uri "$net/subnets" -Method Post -ContentType "application/json" -Body "{`"vpc_id`":`"$($vpc.id)`",`"cidr`":`"10.0.1.0/24`"}"
Write-Host "vpc=$($vpc.id) subnet=$($subnet.id)"

# 2. Launch two instances
$i1 = Invoke-RestMethod -Uri "$reg/instances" -Method Post
$i2 = Invoke-RestMethod -Uri "$reg/instances" -Method Post
Write-Host "i1=$($i1.id) i2=$($i2.id)"

# 3. Assign both to subnet
Invoke-RestMethod -Uri "$net/instances/$($i1.id)/subnet" -Method Put -ContentType "application/json" -Body "{`"subnet_id`":`"$($subnet.id)`"}" | Out-Null
Invoke-RestMethod -Uri "$net/instances/$($i2.id)/subnet" -Method Put -ContentType "application/json" -Body "{`"subnet_id`":`"$($subnet.id)`"}" | Out-Null
Write-Host "instances assigned to subnet"

# 4. Submit ping job on i1
$job = Invoke-RestMethod -Uri "$sch/jobs" -Method Post -ContentType "application/json" -Body "{`"command`":`"echo ping from $($i1.id)`",`"instance_id`":`"$($i1.id)`"}"
Write-Host "job=$($job.job_id)"

Write-Host "done — check job status: tinyaws job status $($job.job_id) --wait"
