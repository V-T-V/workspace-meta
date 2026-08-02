@echo off
setlocal
cd /d "%~dp0"

rem 双击入口：CPU 验证模式。命令行可追加 -Production -CUDA 等参数。
where pwsh.exe >nul 2>nul
if errorlevel 1 goto use_windows_powershell

pwsh.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\auto-deploy-llamacpp.ps1" -NoPause %*
goto done

:use_windows_powershell
powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\auto-deploy-llamacpp.ps1" -NoPause %*

:done
set "EXIT_CODE=%ERRORLEVEL%"
echo.
if not "%EXIT_CODE%"=="0" echo 部署失败，退出码：%EXIT_CODE%
echo 请查看上方日志；按任意键关闭窗口。
pause >nul
exit /b %EXIT_CODE%
