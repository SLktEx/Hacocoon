@echo off
setlocal

set "HACO_WINDOWS=%~dp0haco-windows.ps1"
if not exist "%HACO_WINDOWS%" (
    echo haco: Windows launcher helper is missing: %HACO_WINDOWS% 1>&2
    exit /b 1
)

powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%HACO_WINDOWS%" %*
exit /b %ERRORLEVEL%
