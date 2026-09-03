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

rem The short-lived Windows-side haco launcher is no longer part of Hacocoon.
rem Remove files left by older installers so upgrades do not retain a native
rem haco command. A stale empty PATH entry is harmless and can be cleaned later.
if defined LOCALAPPDATA (
    del /f /q "%LOCALAPPDATA%\Hacocoon\bin\haco.cmd" >nul 2>nul
    del /f /q "%LOCALAPPDATA%\Hacocoon\bin\haco-windows.ps1" >nul 2>nul
    del /f /q "%LOCALAPPDATA%\Hacocoon\bin\INSTANCE" >nul 2>nul
    rmdir "%LOCALAPPDATA%\Hacocoon\bin" >nul 2>nul
)

powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%INSTALLER%" %*
set "HACO_INSTALL_EXIT=%ERRORLEVEL%"

if not "%HACO_INSTALL_EXIT%"=="0" (
    echo.
    echo Hacocoon installation failed with exit code %HACO_INSTALL_EXIT%. 1>&2
    exit /b %HACO_INSTALL_EXIT%
)

exit /b 0
