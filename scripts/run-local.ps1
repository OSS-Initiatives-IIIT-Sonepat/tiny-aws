# Start the full tiny-aws stack in separate windows.
# Run from repo root: .\scripts\run-local.ps1

$root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)

Write-Host "Starting tiny-aws stack..."
Write-Host "Order: registry -> ec2-agent -> object-store -> scheduler"
Write-Host ""

Start-Process powershell -ArgumentList "-NoExit", "-Command", "cd '$root\control-plane\registry'; Write-Host 'Registry (:9000)'; go run ."
Start-Sleep -Seconds 3

Start-Process powershell -ArgumentList "-NoExit", "-Command", "cd '$root\data-plane\compute\ec2-agent'; Write-Host 'EC2 Agent (:8080)'; cargo run"
Start-Sleep -Seconds 2

Start-Process powershell -ArgumentList "-NoExit", "-Command", "cd '$root\data-plane\storage\object-store'; Write-Host 'Object Store (:7001)'; cargo run"
Start-Sleep -Seconds 2

Start-Process powershell -ArgumentList "-NoExit", "-Command", "cd '$root\control-plane\scheduler'; Write-Host 'Scheduler (:9001)'; go run ."

Write-Host ""
Write-Host "Services:"
Write-Host "  registry      http://127.0.0.1:9000/health"
Write-Host "  ec2-agent     http://127.0.0.1:8080/health"
Write-Host "  object-store  http://127.0.0.1:7001/objects"
Write-Host "  scheduler     http://127.0.0.1:9001/health"
Write-Host ""
Write-Host "Run smoke test: .\tests\integration\smoke-test.ps1"
