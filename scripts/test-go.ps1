# Run all Go unit tests across the repository.
# Usage: .\scripts\test-go.ps1

$ErrorActionPreference = "Stop"

$modules = @(
    "control-plane\registry",
    "control-plane\scheduler",
    "control-plane\messaging\sqs",
    "control-plane\messaging\sns",
    "control-plane\networking\vpc",
    "control-plane\controller",
    "control-plane\api",
    "control-plane\metadata",
    "control-plane\cli",
    "data-plane\compute\lambda-runtime",
    "data-plane\networking\load-balancer"
)

$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
if (-not $root) { $root = (Get-Location).Path }

$pass = 0
$fail = 0

foreach ($mod in $modules) {
    $dir = Join-Path $root $mod
    if (-not (Test-Path $dir)) {
        Write-Host "SKIP: $mod (not found)" -ForegroundColor Yellow
        continue
    }
    Write-Host "=== testing $mod ===" -ForegroundColor Cyan
    Push-Location $dir
    try {
        go test -count=1 ./...
        if ($LASTEXITCODE -eq 0) {
            $pass++
        } else {
            $fail++
        }
    } catch {
        $fail++
    }
    Pop-Location
    Write-Host ""
}

Write-Host "=== $pass passed, $fail failed ===" -ForegroundColor $(if ($fail -gt 0) { "Red" } else { "Green" })
if ($fail -gt 0) { exit 1 }
