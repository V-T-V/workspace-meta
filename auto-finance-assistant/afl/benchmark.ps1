# benchmark.ps1 - Automated speed test (UTF-8 safe, multi-round support)
# ============================================================
# Usage:
#   .\benchmark.ps1                # Full test
#   .\benchmark.ps1 -Quick         # Quick mode (3 questions)
#   .\benchmark.ps1 -Rounds 3      # 3 rounds per question
#   .\benchmark.ps1 -Backend ollama
# ============================================================

param(
    [ValidateSet("llamacpp", "ollama", "auto")]
    [string]$Backend = "auto",
    [switch]$Quick,
    [int]$Rounds = 1,
    [int]$TimeoutSec = 300,
    [string]$ServiceUrl = "http://127.0.0.1:8080"
)

$ErrorActionPreference = "Continue"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$Root = Split-Path -Parent $ScriptDir
if (-not $Root) { $Root = $ScriptDir }

# ---- Force UTF-8 encoding for all HTTP calls (fix PS5.x Chinese bug) ----
$OutputEncoding = [System.Text.Encoding]::UTF8
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8

function WS($m) { Write-Host "`n== $m ==" -ForegroundColor Yellow }
function WOK($m) { Write-Host "  [OK] $m" -ForegroundColor Green }
function WE($m) { Write-Host "  [FAIL] $m" -ForegroundColor Red }
function WW($m) { Write-Host "  [WARN] $m" -ForegroundColor DarkYellow }

Write-Host "============================================" -ForegroundColor Cyan
Write-Host "  Auto Finance Assistant - Speed Test" -ForegroundColor Cyan
Write-Host "============================================" -ForegroundColor Cyan

# ---- Test questions ----
$TestQuestions = @(
    @{Category="Cold";     Question="新车贷款最低首付比例是多少"}
    @{Category="Policy";   Question="申请汽车贷款需要准备哪些材料"}
    @{Category="Rate";     Question="当前汽车金融贷款利率是多少"}
    @{Category="Process";  Question="从申请到放款整个流程需要多长时间"}
    @{Category="Repay";    Question="等额本息和等额本金有什么区别"}
    @{Category="FAQ";      Question="你好"}
    @{Category="Reject";   Question="今天天气怎么样"}
    @{Category="Comply";   Question="保证一定能通过审批吗"}
    @{Category="Calc";     Question="贷款20万年利率4.5%分36期等额本息月供多少"}
)

if ($Quick) {
    $TestQuestions = @($TestQuestions | Where-Object { $_.Category -in @("Cold", "Policy", "FAQ") })
    $Rounds = 1
}

# ---- Send request with UTF-8 safe HttpClient (fix P1 encoding bug) ----
function Send-BenchRequest($question) {
    $wallStart = Get-Date
    try {
        # Use .NET HttpClient for reliable UTF-8 encoding
        $json = '{"question":"' + $question.Replace('"','\"') + '"}'
        $bytes = [System.Text.Encoding]::UTF8.GetBytes($json)
        $client = [System.Net.Http.HttpClient]::new()
        $client.Timeout = [TimeSpan]::FromSeconds($TimeoutSec)
        $content = [System.Net.Http.ByteArrayContent]::new($bytes)
        $content.Headers.ContentType = [System.Net.Http.Headers.MediaTypeHeaderValue]::new("application/json")
        $content.Headers.ContentType.CharSet = "utf-8"
        $response = $client.PostAsync("$ServiceUrl/api/chat", $content).Result
        $responseJson = $response.Content.ReadAsStringAsync().Result
        $client.Dispose()

        $resp = $responseJson | ConvertFrom-Json
        $wallMs = [math]::Round(((Get-Date) - $wallStart).TotalMilliseconds)
        $genMs  = $resp.durationMs
        $compTok = $resp.completionTokens
        $promptTok = $resp.promptTokens
        $tps = if ($compTok -gt 0 -and $genMs -gt 0) {
            [math]::Round($compTok / ($genMs / 1000.0), 1)
        } else { 0 }
        return [PSCustomObject]@{
            Question         = $question
            Intent           = $resp.intent
            WallMs           = [int]$wallMs
            GenMs            = [int]$genMs
            PromptTokens     = [int]$promptTok
            CompletionTokens = [int]$compTok
            TokensPerSec     = [double]$tps
            AnswerPreview    = ($resp.answer -replace "`n", " ")
            Error            = $null
        }
    } catch {
        $wallMs = [math]::Round(((Get-Date) - $wallStart).TotalMilliseconds)
        return [PSCustomObject]@{
            Question = $question; Intent = "error"; WallMs = [int]$wallMs
            GenMs = 0; PromptTokens = 0; CompletionTokens = 0; TokensPerSec = 0
            AnswerPreview = ""; Error = $_.Exception.Message
        }
    }
}

# ---- Detect service ----
function Get-BackendInfo {
    try {
        $bytes = [System.Text.Encoding]::UTF8.GetBytes('{}')
        $client = [System.Net.Http.HttpClient]::new()
        $client.Timeout = [TimeSpan]::FromSeconds(5)
        $healthJson = $client.GetStringAsync("$ServiceUrl/api/health").Result
        $client.Dispose()
        $health = $healthJson | ConvertFrom-Json
        return @{ Backend = $health.backend; Model = $health.model; Status = $health.status; Version = $health.version }
    } catch { return $null }
}

# ---- Run test round ----
function Invoke-BenchRound($label) {
    WS $label
    $results = @()
    foreach ($tq in $TestQuestions) {
        $q = $tq.Question
        $cat = $tq.Category
        $shortQ = $q.Substring(0, [math]::Min(40, $q.Length))
        Write-Host "  [$cat] $shortQ " -NoNewline -ForegroundColor Gray

        $best = $null
        # Fix P2: use different variable name for loop counter vs result
        for ($round = 0; $round -lt [int]$Rounds; $round++) {
            $result = Send-BenchRequest $q
            if (-not $best -or $result.WallMs -lt $best.WallMs) { $best = $result }
        }

        $icon = if ($best.Error) { "X" }
                elseif ($best.TokensPerSec -gt 0) { "V" }
                elseif ($best.Intent -match "guard|faq|compliance") { "*" }
                else { "?" }

        $tpsStr = if ($best.TokensPerSec -gt 0) { "$($best.TokensPerSec) tok/s" } else { "-" }
        $timeStr = if ($best.GenMs -gt 0) { "$($best.GenMs)ms" } else { "$($best.WallMs)ms" }
        $color = if ($best.Error) { "Red" } elseif ($best.TokensPerSec -gt 20) { "Green" } elseif ($best.WallMs -lt 200) { "Cyan" } else { "Yellow" }
        Write-Host " $icon $timeStr $tpsStr [$($best.Intent)]" -ForegroundColor $color

        $best | Add-Member -NotePropertyName Category -NotePropertyValue $cat -Force
        $results += $best
    }
    return $results
}

# ============================================================
# MAIN
# ============================================================

WS "Service detection"
$info = Get-BackendInfo
if (-not $info) {
    WE "Cannot connect to $ServiceUrl"
    Write-Host "  Start: .\auto-finance-assistant.exe -config config.yaml run" -ForegroundColor Yellow
    exit 1
}
Log "Backend: $($info.Backend)  Model: $($info.Model)  Status: $($info.Status)"

# Warmup
WS "Warmup"
WW "Sending warmup request..."
$warmup = Send-BenchRequest "Hello, please introduce auto finance briefly"
Log "Warmup done (Intent=$($warmup.Intent), $($warmup.WallMs)ms)"

# Test
WS "Testing ($Rounds round/question)"
$allResults = Invoke-BenchRound "Running"

# Save logs
$LogDir = Join-Path $Root "data\logs"
if (-not (Test-Path $LogDir)) { New-Item -ItemType Directory -Path $LogDir -Force | Out-Null }
$Timestamp = Get-Date -Format "yyyyMMdd-HHmmss"
$JsonLog = Join-Path $LogDir "benchmark-$Timestamp.json"
$MdLog = Join-Path $LogDir "benchmark-$Timestamp.md"
$allResults | ConvertTo-Json -Depth 3 | Out-File $JsonLog -Encoding utf8

# Summary
WS "Summary"
$modelResults = $allResults | Where-Object { $_.TokensPerSec -gt 0 }
if ($modelResults.Count -gt 0) {
    $avgTps = [math]::Round(($modelResults | Measure-Object -Property TokensPerSec -Average).Average, 1)
    $avgWall = [math]::Round(($modelResults | Measure-Object -Property WallMs -Average).Average, 0)
    Write-Host "  Avg TPS:  $avgTps tok/s" -ForegroundColor Green
    Write-Host "  Avg Wall: $avgWall ms" -ForegroundColor Green
}
$guardResults = $allResults | Where-Object { $_.Intent -match "guard|compliance" }
if ($guardResults.Count -gt 0) {
    $guardAvg = [math]::Round(($guardResults | Measure-Object -Property WallMs -Average).Average, 0)
    Write-Host "  Guard avg: $guardAvg ms" -ForegroundColor Cyan
}

Write-Host ""
Write-Host "  JSON: $JsonLog" -ForegroundColor Green
Write-Host "  MD:   $MdLog" -ForegroundColor Green
