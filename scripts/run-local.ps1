# Start the full tiny-aws stack in separate windows.
# Run from repo root: .\scripts\run-local.ps1

$root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)

Write-Host "Building Rust crates first (may take a few minutes on first run)..."
Push-Location "$root\data-plane\compute\ec2-agent"; cargo build 2>&1 | Select-Object -Last 1; Pop-Location
Push-Location "$root\data-plane\storage\object-store"; cargo build 2>&1 | Select-Object -Last 1; Pop-Location
Write-Host "Rust build done."
Write-Host ""

Write-Host "Starting tiny-aws stack..."

# core services
Start-Process powershell -ArgumentList "-NoExit", "-Command", "cd '$root\control-plane\registry'; Write-Host 'Registry (:9000)'; go run ."
Start-Sleep -Seconds 3

Start-Process powershell -ArgumentList "-NoExit", "-Command", "cd '$root\data-plane\compute\ec2-agent'; Write-Host 'EC2 Agent (:8080)'; .\target\debug\ec2-agent.exe"
Start-Sleep -Seconds 3

Start-Process powershell -ArgumentList "-NoExit", "-Command", "cd '$root\data-plane\storage\object-store'; Write-Host 'Object Store (:7001)'; .\target\debug\object-store.exe"
Start-Sleep -Seconds 5

Start-Process powershell -ArgumentList "-NoExit", "-Command", "cd '$root\control-plane\scheduler'; Write-Host 'Scheduler (:9001)'; go run ."
Start-Sleep -Seconds 2

# optional services
Start-Process powershell -ArgumentList "-NoExit", "-Command", "cd '$root\control-plane\controller'; Write-Host 'Controller (:9002)'; go run ."
Start-Process powershell -ArgumentList "-NoExit", "-Command", "cd '$root\data-plane\networking\load-balancer'; Write-Host 'LB (:8088)'; go run ."

Write-Host ""
Write-Host "Services:"
Write-Host "  registry      http://127.0.0.1:9000/health"
Write-Host "  ec2-agent     http://127.0.0.1:8080/health"
Write-Host "  object-store  http://127.0.0.1:7001/health"
Write-Host "  scheduler     http://127.0.0.1:9001/health"
Write-Host "  controller    http://127.0.0.1:9002/health"
Write-Host "  load-balancer http://127.0.0.1:8088/health"
Write-Host ""
Write-Host "Run smoke test: .\tests\integration\smoke-test.ps1"
