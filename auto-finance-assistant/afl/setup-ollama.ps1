# setup-ollama.ps1 - Offline-first install (Ollama backend)
# ============================================================
# Usage:
#   .\setup-ollama.ps1              # Auto-detect drive (D > C)
#   .\setup-ollama.ps1 -Drive E     # Install to E: drive
# ============================================================

param(
    [string]$Drive = "",
    [switch]$CPU
)

$ErrorActionPreference = "Stop"
$Src = Split-Path -Parent $MyInvocation.MyCommand.Path

function WS($m) { Write-Host "`n== $m ==" -ForegroundColor Yellow }
function WOK($m) { Write-Host "  [OK] $m" -ForegroundColor Green }
function WE($m) { Write-Host "  [FAIL] $m" -ForegroundColor Red }
function WW($m) { Write-Host "  [WARN] $m" -ForegroundColor DarkYellow }

Write-Host "============================================" -ForegroundColor Cyan
Write-Host "  Auto Finance Assistant - Ollama" -ForegroundColor Cyan
Write-Host "============================================" -ForegroundColor Cyan

# ============================================================
# 0. Select drive
# ============================================================
WS "0. Select drive"
if ($Drive -eq "") {
    if (Test-Path "D:\") { $Drv = "D" } else { $Drv = "C" }
} else {
    $Drv = $Drive.ToUpper()
    if (-not (Test-Path "${Drv}:\")) { WE "Drive ${Drv}: not found"; exit 1 }
}

$Dst = "${Drv}:\AutoFinanceAssistant"
WS "Install to $Dst (drive ${Drv}:)"
New-Item -ItemType Directory -Path $Dst -Force | Out-Null

# ============================================================
# 1. Copy files
# ============================================================
WS "1. Copy files"

Write-Host "  Stopping leftover processes..." -ForegroundColor Gray
Get-Process -Name "ollama" -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Get-Process -Name "auto-finance-assistant" -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 2

Copy-Item (Join-Path $Src "auto-finance-assistant.exe") $Dst -Force
WOK "Main EXE"

# ============================================================
# 2. Install & start Ollama
# ============================================================
WS "2. Setup Ollama"

$ollamaExe = ""
try { $ollamaExe = (Get-Command ollama -ErrorAction Stop).Source } catch {}

if ($ollamaExe -eq "") {
    $ollamaSetup = Join-Path $Src "OllamaSetup.exe"
    if (Test-Path $ollamaSetup) {
        Write-Host "  Installing Ollama..." -ForegroundColor Gray
        Start-Process -FilePath $ollamaSetup -ArgumentList "/S" -Wait
        Start-Sleep -Seconds 5
        $env:PATH = [System.Environment]::GetEnvironmentVariable("PATH","Machine") + ";" + [System.Environment]::GetEnvironmentVariable("PATH","User")
        try { $ollamaExe = (Get-Command ollama -ErrorAction Stop).Source } catch {}
    } else {
        WE "OllamaSetup.exe not found and ollama not in PATH"
        WW "Download from: https://ollama.com/download/windows"
        exit 1
    }
}
WOK "Ollama: $ollamaExe"

# Set OLLAMA_MODELS to target drive
$env:OLLAMA_MODELS = "${Drv}:\OllamaModels"
[System.Environment]::SetEnvironmentVariable("OLLAMA_MODELS", $env:OLLAMA_MODELS, "User")
New-Item -ItemType Directory -Path $env:OLLAMA_MODELS -Force | Out-Null

# Start Ollama serve
$ollamaRunning = $false
try {
    $null = Invoke-RestMethod -Uri "http://127.0.0.1:11434/api/version" -TimeoutSec 2 -ErrorAction Stop
    $ollamaRunning = $true
    WOK "Ollama already running"
} catch {}

if (-not $ollamaRunning) {
    Start-Process -FilePath $ollamaExe -ArgumentList "serve" -WindowStyle Hidden
    Start-Sleep -Seconds 5
    for ($i = 0; $i -lt 15; $i++) {
        try {
            $null = Invoke-RestMethod -Uri "http://127.0.0.1:11434/api/version" -TimeoutSec 3 -ErrorAction Stop
            $ollamaRunning = $true; break
        } catch {}
        Start-Sleep -Seconds 2
    }
    if ($ollamaRunning) { WOK "Ollama started" }
    else { WE "Ollama startup timeout"; exit 1 }
}

# ============================================================
# 3. Check & pull models
# ============================================================
WS "3. Check models"

$chatModel = "qwen3:4b"
$embedModel = "nomic-embed-text"

# Get existing models
$existingModels = @()
try {
    $tags = Invoke-RestMethod -Uri "http://127.0.0.1:11434/api/tags" -TimeoutSec 10
    $existingModels = $tags.models | ForEach-Object { $_.name }
} catch {}
Write-Host "  Existing models: $($existingModels -join ', ')" -ForegroundColor Gray

# Check chat model
$hasChat = ($existingModels | Where-Object { $_ -match "qwen3\.5.*4b" }).Count -gt 0
if (-not $hasChat) {
    Write-Host "  Pulling $chatModel (~2.6GB, needs internet)..." -ForegroundColor Gray
    try {
        & $ollamaExe pull $chatModel 2>&1 | ForEach-Object { Write-Host "    $_" -ForegroundColor DarkGray }
        if ($LASTEXITCODE -ne 0) { throw "pull failed" }
        $hasChat = $true
    } catch {
        WW "qwen3:4b pull failed, trying qwen2.5:3b..."
        $chatModel = "qwen2.5:3b"
        try { & $ollamaExe pull $chatModel 2>&1 | Out-Null; $hasChat = $true } catch {}
    }
}
if ($hasChat) { WOK "Chat model: $chatModel" } else { WE "No chat model available"; exit 1 }

# Check embedding model
$hasEmbed = ($existingModels | Where-Object { $_ -match "nomic-embed" }).Count -gt 0
if (-not $hasEmbed) {
    Write-Host "  Pulling $embedModel (~274MB)..." -ForegroundColor Gray
    try {
        & $ollamaExe pull $embedModel 2>&1 | ForEach-Object { Write-Host "    $_" -ForegroundColor DarkGray }
        $hasEmbed = $true
    } catch {
        WW "Embedding model pull failed, RAG disabled (chat still works)"
    }
}
if ($hasEmbed) { WOK "Embed model: $embedModel" }

# ============================================================
# 4. Generate config.yaml
# ============================================================
WS "4. Generate config"

$configYaml = @"
server:
  host: 127.0.0.1
  port: 8080
ollama:
  backend: ollama
  base_url: http://127.0.0.1:11434
  chat_model: ${chatModel}
  embedding_model: ${embedModel}
  request_timeout_seconds: 180
generation:
  num_thread: 8
  context_size: 4096
  max_output_tokens: 0
  temperature: 0.7
queue:
  generation_concurrency: 1
  maximum_waiting: 10
  request_timeout_seconds: 180
storage:
  database_path: ./data/assistant.db
  document_path: ./data/documents
  temp_path: ./data/temp
  backup_path: ./data/backups
logging:
  level: info
  directory: ./data/logs
  max_file_size_mb: 20
  max_files: 7
  retain_days: 30
"@

$configPath = Join-Path $Dst "config.yaml"
$configYaml | Out-File -FilePath $configPath -Encoding utf8
WOK "config.yaml (backend: ollama)"

# ============================================================
# 5. Start auto-finance-assistant
# ============================================================
WS "5. Start application"

$existing = Get-Process -Name "auto-finance-assistant" -ErrorAction SilentlyContinue
if ($existing) {
    WW "auto-finance-assistant already running (PID $($existing.Id))"
} else {
    $exePath = Join-Path $Dst "auto-finance-assistant.exe"
    Start-Process -FilePath $exePath -ArgumentList "-config", $configPath, "run" -WorkingDirectory $Dst
    Start-Sleep -Seconds 8

    $appReady = $false
    for ($i = 0; $i -lt 15; $i++) {
        try {
            $h = Invoke-RestMethod -Uri "http://127.0.0.1:8080/api/health" -TimeoutSec 3 -ErrorAction Stop
            if ($h.status) { $appReady = $true; break }
        } catch {}
        Start-Sleep -Seconds 2
    }
    if ($appReady) { WOK "Application started" }
    else { WW "Application may still be starting..." }
}

# ============================================================
# Done
# ============================================================
Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host "  Installation Complete! (Ollama)" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
Write-Host ""
Write-Host "  Browser: http://127.0.0.1:8080" -ForegroundColor White -BackgroundColor DarkBlue
Write-Host "  Backend: ollama | Model: $chatModel" -ForegroundColor White
Write-Host ""
Write-Host "  Stop:"
Write-Host "    Get-Process ollama,auto-finance-assistant | Stop-Process -Force"
