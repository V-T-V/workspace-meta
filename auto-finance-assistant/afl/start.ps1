# start.ps1 - Start auto-finance-assistant (foreground, for debugging)
# Usage: .\start.ps1
$ErrorActionPreference = "Stop"
$Dir = Split-Path -Parent $MyInvocation.MyCommand.Path
$Exe = Join-Path $Dir "auto-finance-assistant.exe"
$Cfg = Join-Path $Dir "config.yaml"

if (-not (Test-Path $Exe)) { Write-Host "EXE not found: $Exe" -ForegroundColor Red; exit 1 }
if (-not (Test-Path $Cfg)) { Write-Host "Config not found: $Cfg" -ForegroundColor Red; exit 1 }

Write-Host "Starting...  http://127.0.0.1:8080  (Ctrl+C to stop)" -ForegroundColor Cyan
& $Exe -config $Cfg run
