# setup.ps1 - Auto-detect backend dispatcher
# ============================================================
# Usage:
#   .\setup.ps1              # Auto-detect backend + drive
#   .\setup.ps1 -Drive E     # Specify drive
#   .\setup.ps1 -CPU         # Force CPU mode
#
# Or call directly:
#   .\setup-llamacpp.ps1     # llama.cpp only
#   .\setup-ollama.ps1       # Ollama only
# ============================================================

param(
    [string]$Drive = "",
    [switch]$CPU
)

$Src = Split-Path -Parent $MyInvocation.MyCommand.Path

$scriptArgs = @()
if ($Drive -ne "") { $scriptArgs += "-Drive", $Drive }
if ($CPU)         { $scriptArgs += "-CPU" }

# Prefer llama.cpp if binary exists in package
$llamaExe = Join-Path $Src "llamacpp\llama-server.exe"
$ollamaSetup = Join-Path $Src "OllamaSetup.exe"

if (Test-Path $llamaExe) {
    Write-Host "Detected: llama.cpp -> setup-llamacpp.ps1" -ForegroundColor Cyan
    $target = Join-Path $Src "setup-llamacpp.ps1"
} elseif (Test-Path $ollamaSetup) {
    Write-Host "Detected: Ollama -> setup-ollama.ps1" -ForegroundColor Cyan
    $target = Join-Path $Src "setup-ollama.ps1"
} else {
    Write-Host "No backend detected, trying Ollama (if installed)..." -ForegroundColor Yellow
    $target = Join-Path $Src "setup-ollama.ps1"
}

if (-not (Test-Path $target)) {
    Write-Host "[FAIL] Setup script missing: $target" -ForegroundColor Red
    Write-Host "Make sure setup-llamacpp.ps1 and setup-ollama.ps1 are in the same folder." -ForegroundColor Yellow
    exit 1
}

& $target @scriptArgs
