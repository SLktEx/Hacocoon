@echo off
setlocal

set "INSTALLER=%~dp0install-windows.ps1"

where powershell.exe >nul 2>nul
if errorlevel 1 (
    echo Hacocoon installer error: powershell.exe was not found. 1>&2
    exit /b 1
)

if not exist "%INSTALLER%" (
    echo Hacocoon installer error: install-windows.ps1 must be next to install-windows.bat. 1>&2
    exit /b 1
)

powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%INSTALLER%" %*
set "HACO_INSTALL_EXIT=%ERRORLEVEL%"

if not "%HACO_INSTALL_EXIT%"=="0" (
    echo.
    echo Hacocoon installation failed with exit code %HACO_INSTALL_EXIT%. 1>&2
    exit /b %HACO_INSTALL_EXIT%
)

exit /b 0
