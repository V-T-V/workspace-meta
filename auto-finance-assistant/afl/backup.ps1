# backup.ps1 - 手动备份数据库与文档
# 用法：.\scripts\backup.ps1
# 对应原计划第二十节。使用 SQLite Online Backup API（通过服务接口触发）
$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
Set-Location $Root

Write-Host "=== 触发备份（通过 API）===" -ForegroundColor Cyan
$resp = Invoke-RestMethod -Uri "http://127.0.0.1:8080/api/system/backup" -Method Post -ErrorAction SilentlyContinue
if ($resp) {
    Write-Host "备份完成：$($resp.path)" -ForegroundColor Green
} else {
    # 服务未运行时，直接复制文件（需先停止写入）
    Write-Host "服务未运行，直接复制文件..." -ForegroundColor Yellow
    $ts = Get-Date -Format "yyyyMMdd-HHmmss"
    $backupDir = "data\backups\$ts"
    New-Item -ItemType Directory -Path $backupDir -Force | Out-Null
    Copy-Item "data\assistant.db" $backupDir -ErrorAction SilentlyContinue
    Copy-Item "data\assistant.db-wal" $backupDir -ErrorAction SilentlyContinue
    Copy-Item "config.yaml" $backupDir -ErrorAction SilentlyContinue
    Write-Host "文件备份完成：$backupDir" -ForegroundColor Green
    Write-Host "注意：直接复制正在写入的 DB 可能不一致，建议通过服务接口备份" -ForegroundColor Yellow
}
