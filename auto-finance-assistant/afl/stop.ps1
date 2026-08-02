# stop.ps1 - Stop all services (app + llama-server + ollama)
$ErrorActionPreference = "SilentlyContinue"

Write-Host "Stopping..." -ForegroundColor Yellow

# Windows service
$svc = Get-Service -Name "AutoFinanceAssistant" -ErrorAction SilentlyContinue
if ($svc) {
    Stop-Service -Name "AutoFinanceAssistant" -Force
    Write-Host "  Service stopped" -ForegroundColor Green
}

# All processes
Get-Process -Name "auto-finance-assistant" -ErrorAction SilentlyContinue | Stop-Process -Force
Get-Process -Name "llama-server" -ErrorAction SilentlyContinue | Stop-Process -Force
Get-Process -Name "ollama" -ErrorAction SilentlyContinue | Stop-Process -Force

Write-Host "All stopped." -ForegroundColor Green
