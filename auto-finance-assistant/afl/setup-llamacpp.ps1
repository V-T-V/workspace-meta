# setup-llamacpp.ps1 - Offline install (llama.cpp backend)
# ============================================================
# Usage:
#   .\setup-llamacpp.ps1              # Auto-detect drive (D > C), auto GPU
#   .\setup-llamacpp.ps1 -Drive E     # Install to E: drive
#   .\setup-llamacpp.ps1 -CPU         # Force CPU mode
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
Write-Host "  Auto Finance Assistant - llama.cpp" -ForegroundColor Cyan
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
$ModelsDst = "${Drv}:\LlamaModels"
WS "Install to $Dst (drive ${Drv}:)"
New-Item -ItemType Directory -Path $Dst -Force | Out-Null
New-Item -ItemType Directory -Path $ModelsDst -Force | Out-Null

# ============================================================
# 1. Copy files
# ============================================================
WS "1. Copy files"

# Kill leftover processes that may lock files
Write-Host "  Stopping leftover processes..." -ForegroundColor Gray
Get-Process -Name "llama-server" -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Get-Process -Name "auto-finance-assistant" -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 3
$stillRunning = Get-Process -Name "llama-server" -ErrorAction SilentlyContinue
if ($stillRunning) {
    Write-Host "  Force killing llama-server (PID $($stillRunning.Id))..." -ForegroundColor DarkYellow
    taskkill /F /PID $stillRunning.Id 2>$null
    Start-Sleep -Seconds 2
}

# Main EXE
Copy-Item (Join-Path $Src "auto-finance-assistant.exe") $Dst -Force
WOK "Main EXE"

# llama-server + DLLs
$LlamaDst = Join-Path $Dst "llamacpp"
New-Item -ItemType Directory -Path $LlamaDst -Force | Out-Null
$llamaSrc = Join-Path $Src "llamacpp\llama-server.exe"
if (Test-Path $llamaSrc) {
    try {
        Copy-Item (Join-Path $Src "llamacpp\*") $LlamaDst -Force -Recurse -ErrorAction Stop
        WOK "llama-server + DLLs"
    } catch {
        WW "Some files locked, copying individually..."
        Get-ChildItem (Join-Path $Src "llamacpp") | ForEach-Object {
            $target = Join-Path $LlamaDst $_.Name
            try { Copy-Item $_.FullName $target -Force -ErrorAction Stop }
            catch { Write-Host "    Skipped: $($_.Name) (locked)" -ForegroundColor DarkGray }
        }
    }
} else {
    WE "llamacpp\llama-server.exe not found in package"
    exit 1
}

# GGUF models
$modelsSrc = Join-Path $Src "models"
if (Test-Path $modelsSrc) {
    Copy-Item (Join-Path $Src "models\*") $ModelsDst -Force
    WOK "Models copied to $ModelsDst"
} else {
    WW "No models/ directory in package (models must be already at $ModelsDst)"
}

# ============================================================
# 2. Detect GPU
# ============================================================
WS "2. Detect GPU"
$hasNvidia = $false
$ngl = 0
try {
    $gpu = Get-CimInstance Win32_VideoController | Where-Object { $_.Name -match "NVIDIA|RTX|GeForce" }
    if ($gpu) {
        $hasNvidia = $true
        $ngl = 99
        WOK "NVIDIA GPU: $($gpu.Name)"
    }
} catch {}

if ($CPU) {
    $hasNvidia = $false
    $ngl = 0
    WW "CPU mode forced (-CPU flag)"
}
if (-not $hasNvidia) {
    WW "No NVIDIA GPU detected, using CPU inference"
}

# ============================================================
# 3. Find chat model
# ============================================================
WS "3. Select chat model"

$chatModelFile = ""
$chatModel = ""
$ctxSize = 4096
$temp = 0.1

if (Test-Path (Join-Path $ModelsDst "Qwen3-4B-Q4_K_M.gguf")) {
    $chatModelFile = "Qwen3-4B-Q4_K_M.gguf"
    $chatModel = "Qwen3-4B"
    $ctxSize = 4096
    $temp = 0.1
} elseif (Test-Path (Join-Path $ModelsDst "qwen2.5-3b-instruct-q4_k_m.gguf")) {
    $chatModelFile = "qwen2.5-3b-instruct-q4_k_m.gguf"
    $chatModel = "qwen2.5-3b"
    $ctxSize = 4096
    $temp = 0.3
} else {
    $ggufFiles = Get-ChildItem $ModelsDst -Filter "*.gguf" -ErrorAction SilentlyContinue | Where-Object { $_.Name -notmatch "embed" }
    if ($ggufFiles) {
        $chatModelFile = $ggufFiles[0].Name
        $chatModel = $ggufFiles[0].BaseName
        $ctxSize = 4096
        $temp = 0.3
    }
}

if ($chatModelFile -eq "") {
    WE "No chat model (*.gguf) found in $ModelsDst"
    $allFiles = Get-ChildItem $ModelsDst -Filter "*.gguf" -ErrorAction SilentlyContinue
    if ($allFiles) {
        Write-Host "  Available models:" -ForegroundColor Yellow
        $allFiles | ForEach-Object { Write-Host "    $($_.Name)" -ForegroundColor Yellow }
    } else {
        Write-Host "  Directory is empty. Download models from HuggingFace." -ForegroundColor Yellow
    }
    exit 1
}
WOK "Chat model: $chatModelFile"

# ============================================================
# 4. Start llama-server (chat, port 8081)
# ============================================================
WS "4. Start llama-server (chat, port 8081)"

$llamaExe = Join-Path $LlamaDst "llama-server.exe"
$chatPath = Join-Path $ModelsDst $chatModelFile
$chatPort = 8081

$chatRunning = $false
try {
    $null = Invoke-RestMethod -Uri "http://127.0.0.1:${chatPort}/health" -TimeoutSec 2 -ErrorAction Stop
    $chatRunning = $true
    WOK "llama-server already running on port $chatPort"
} catch {}

if (-not $chatRunning) {
    $chatArgs = @("-m", $chatPath, "--port", $chatPort, "--host", "127.0.0.1", "-c", $ctxSize, "--mmap")
    if ($ngl -gt 0) {
        $chatArgs += @("-ngl", $ngl, "--mlock")
        Write-Host "  Starting GPU mode (-ngl $ngl, --mlock)..." -ForegroundColor Gray
    } else {
        $chatArgs += @("-t", "8")
        Write-Host "  Starting CPU mode (-t 8)..." -ForegroundColor Gray
    }
    Start-Process -FilePath $llamaExe -ArgumentList $chatArgs -WorkingDirectory $LlamaDst -WindowStyle Hidden

    # Wait for ready
    if ($ngl -gt 0) {
        $maxWait = 30
        Write-Host "  GPU loading (~15-30s)..." -ForegroundColor Gray
    } else {
        $maxWait = 60
        Write-Host "  CPU loading (~60-120s)..." -ForegroundColor Gray
    }
    $ready = $false
    for ($i = 0; $i -lt $maxWait; $i++) {
        Start-Sleep -Seconds 2
        try {
            $null = Invoke-RestMethod -Uri "http://127.0.0.1:${chatPort}/health" -TimeoutSec 3 -ErrorAction Stop
            $ready = $true; break
        } catch {}
        if ($i -gt 0 -and ($i % 5 -eq 0)) {
            $elapsed = $i * 2
            Write-Host "  [${elapsed}s] Still loading..." -ForegroundColor DarkGray
        }
        $proc = Get-Process -Name "llama-server" -ErrorAction SilentlyContinue
        if (-not $proc -and $i -gt 3) {
            WE "llama-server process exited unexpectedly"
            Write-Host "  Possible causes: GPU driver issue, insufficient VRAM, missing DLL" -ForegroundColor Yellow
            Write-Host "  Try manually: $llamaExe -m `"$chatPath`" --port $chatPort --host 127.0.0.1 -c $ctxSize --mmap" -ForegroundColor Yellow
            exit 1
        }
    }
    if ($ready) { WOK "llama-server (chat) started on port $chatPort" }
    else { WE "llama-server startup timeout"; exit 1 }
}

# ============================================================
# 5. Start llama-server (embedding, port 8082)
# ============================================================
WS "5. Start llama-server (embedding, port 8082)"

$embedPort = 8082
$embedRunning = $false
try {
    $null = Invoke-RestMethod -Uri "http://127.0.0.1:${embedPort}/health" -TimeoutSec 2 -ErrorAction Stop
    $embedRunning = $true
    WOK "Embedding server already running"
} catch {}

$embedModelFile = "nomic-embed-text-v1.5.Q8_0.gguf"
$embedPath = Join-Path $ModelsDst $embedModelFile

if (-not $embedRunning) {
    if ((Test-Path $embedPath)) {
        $embedArgs = @("-m", $embedPath, "--port", $embedPort, "--host", "127.0.0.1", "--embedding", "-c", "2048")
        if ($ngl -gt 0) { $embedArgs += @("-ngl", "99") } else { $embedArgs += @("-t", "4") }
        Start-Process -FilePath $llamaExe -ArgumentList $embedArgs -WorkingDirectory $LlamaDst -WindowStyle Hidden
        Start-Sleep -Seconds 8
        WOK "Embedding server started on port $embedPort"
    } else {
        WW "Embedding model not found ($embedModelFile), vector RAG disabled"
    }
}

# ============================================================
# 6. Generate config.yaml
# ============================================================
WS "6. Generate config"

$configYaml = @"
server:
  host: 127.0.0.1
  port: 8080
ollama:
  backend: llamacpp
  base_url: http://127.0.0.1:${chatPort}/v1
  embedding_base_url: http://127.0.0.1:${embedPort}/v1
  chat_model: ${chatModel}
  embedding_model: nomic-embed-text-v1.5.Q8_0
  request_timeout_seconds: 180
generation:
  num_thread: 8
  context_size: ${ctxSize}
  max_output_tokens: 400
  temperature: ${temp}
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
WOK "config.yaml (backend: llamacpp)"

# ============================================================
# 7. Start auto-finance-assistant
# ============================================================
WS "7. Start application"

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
Write-Host "  Installation Complete! (llama.cpp)" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
Write-Host ""
Write-Host "  Browser: http://127.0.0.1:8080" -ForegroundColor White -BackgroundColor DarkBlue
Write-Host "  Backend: llamacpp | Model: $chatModel" -ForegroundColor White
Write-Host "  Chat port: $chatPort | Embed port: $embedPort" -ForegroundColor White
Write-Host ""
Write-Host "  Stop:"
Write-Host "    Get-Process llama-server,auto-finance-assistant | Stop-Process -Force"
Write-Host ""
Write-Host "  Restart:"
Write-Host "    cd $Dst"
Write-Host "    .\auto-finance-assistant.exe -config config.yaml run"
