# gpu-mesh Agent 批量部署脚本（PowerShell）
#
# 从 CSV 批量远程安装 Agent 到多台 Windows 机器。
# 适用场景：上百台异地 4060 机器一次性纳管。
#
# 用法：
#   .\batch-install-agent.ps1 -Csv agents.csv -Relay "wss://gpu-mesh.yourdomain.com" -Token "xxx"
#
# CSV 格式（agents.csv）：
#   host,user,password,agent_id
#   192.168.1.101,admin,P@ssw0rd,gpu-bj-001
#   10.0.0.52,administrator,xxx,gpu-sh-014
#
# 前置条件：
#   - 运行机器需能 WinRM/SSH 连到目标机器
#   - 目标机器需已装 NVIDIA 驱动（nvidia-smi 可用）
#   - 目标机器需管理员权限

param(
    [Parameter(Mandatory=$true)]
    [string]$Csv,                 # 主机清单 CSV
    [Parameter(Mandatory=$true)]
    [string]$Relay,               # 如 wss://gpu-mesh.yourdomain.com
    [Parameter(Mandatory=$true)]
    [string]$Token,
    [string]$AgentExe = "",       # gpu-mesh-agent.exe 路径（空=从 VPS 下载）
    [string]$AgentUrl = ""        # Agent 下载地址（AgentExe 为空时用）
)

$ErrorActionPreference = "Continue"
$Results = @()

Write-Host "=== GPU Mesh Agent 批量部署 ===" -ForegroundColor Cyan
Write-Host "目标: $Csv | Relay: $Relay"
Write-Host ""

# 读取 CSV
if (-not (Test-Path $Csv)) {
    Write-Host "✗ CSV 文件不存在: $Csv" -ForegroundColor Red
    exit 1
}
$hosts = Import-Csv $Csv
Write-Host "共 $($hosts.Count) 台机器待部署" -ForegroundColor Yellow
Write-Host ""

# 逐台部署
foreach ($h in $hosts) {
    $host_ip = $h.host
    $user = $h.user
    $agent_id = $h.agent_id
    if (-not $agent_id) { $agent_id = $host_ip }

    Write-Host "[$($host_ip)] 部署 $agent_id ..." -ForegroundColor Cyan -NoNewline

    try {
        # 构建远程执行的脚本块
        $remoteScript = {
            param($Relay, $Token, $AgentID, $AgentExe, $AgentUrl)

            # 1. 检查管理员
            $isAdmin = ([Security.Principal.WindowsPrincipal] `
                [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole(`
                [Security.Principal.WindowsBuiltInRole]::Administrator)
            if (-not $isAdmin) { throw "需要管理员权限" }

            # 2. 获取或下载 Agent exe
            $installDir = "C:\Program Files\gpu-mesh"
            $exePath = "$installDir\gpu-mesh-agent.exe"
            if (-not (Test-Path $installDir)) {
                New-Item -ItemType Directory -Path $installDir -Force | Out-Null
            }

            if ($AgentExe -and (Test-Path $AgentExe)) {
                Copy-Item $AgentExe $exePath -Force
            } elseif ($AgentUrl) {
                Invoke-WebRequest $AgentUrl -OutFile $exePath
            } else {
                throw "无 AgentExe 也无 AgentUrl"
            }

            # 3. 卸载旧服务
            if (Get-Service "gpu-mesh-agent" -ErrorAction SilentlyContinue) {
                Stop-Service "gpu-mesh-agent" -Force -ErrorAction SilentlyContinue
                & $exePath uninstall 2>$null
                Start-Sleep 2
            }

            # 4. 检查 NVIDIA 驱动
            $hasNvidia = $false
            try { $hasNvidia = [bool](Get-Command nvidia-smi -ErrorAction Stop) } catch {}

            # 5. 安装服务
            Push-Location $installDir
            & $exePath install -relay $Relay -id $AgentID -token $Token 2>&1 | Out-Null
            Pop-Location

            # 6. 启动
            Start-Service "gpu-mesh-agent"
            Start-Sleep 3

            $svc = Get-Service "gpu-mesh-agent"
            if ($svc.Status -eq "Running") {
                @{ Success = $true; Nvidia = $hasNvidia; Msg = "服务运行中" }
            } else {
                @{ Success = $false; Nvidia = $hasNvidia; Msg = "服务未运行: $($svc.Status)" }
            }
        }

        # 远程执行（WinRM，需目标机器开启 5985/5986）
        $cred = New-Object System.Management.Automation.PSCredential($user, (ConvertTo-SecureString $h.password -AsPlainText -Force))
        $result = Invoke-Command -ComputerName $host_ip -Credential $cred -ScriptBlock $remoteScript `
            -ArgumentList $Relay, $Token, $agent_id, $AgentExe, $AgentUrl -ErrorAction Stop

        if ($result.Success) {
            Write-Host " ✓ 成功" -ForegroundColor Green
            if (-not $result.Nvidia) {
                Write-Host "   ⚠ 未检测到 nvidia-smi" -ForegroundColor Yellow
            }
            $Results += [PSCustomObject]@{ Host=$host_ip; AgentID=$agent_id; Status="成功"; Nvidia=$result.Nvidia }
        } else {
            Write-Host " ✗ $($result.Msg)" -ForegroundColor Red
            $Results += [PSCustomObject]@{ Host=$host_ip; AgentID=$agent_id; Status="失败: $($result.Msg)"; Nvidia=$result.Nvidia }
        }
    } catch {
        Write-Host " ✗ $($_.Exception.Message)" -ForegroundColor Red
        $Results += [PSCustomObject]@{ Host=$host_ip; AgentID=$agent_id; Status="连接失败: $($_.Exception.Message)"; Nvidia=$false }
    }
}

# 汇总
Write-Host ""
Write-Host "=== 部署汇总 ===" -ForegroundColor Cyan
$Results | Format-Table -AutoSize
$ok = ($Results | Where-Object Status -eq "成功").Count
$fail = ($Results | Where-Object Status -ne "成功").Count
Write-Host ""
Write-Host "成功: $ok / $($Results.Count)    失败: $fail" -ForegroundColor $(if($fail -eq 0){'Green'}else{'Yellow'})

# 导出结果
$outCsv = "deploy-result-$(Get-Date -Format 'yyyyMMdd-HHmm').csv"
$Results | Export-Csv $outCsv -NoTypeInformation
Write-Host "详细结果: $outCsv"
