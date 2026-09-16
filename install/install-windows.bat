@echo off
setlocal
set "HACO_INSTALL_EXIT=1"

set "INSTALLER=%~dp0install-windows.ps1"

where powershell.exe >nul 2>nul
if errorlevel 1 (
    echo Hacocoon installer error: powershell.exe was not found. 1>&2
    goto finish
)

if not exist "%INSTALLER%" (
    echo Hacocoon installer error: install-windows.ps1 must be next to install-windows.bat. 1>&2
    goto finish
)

powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%INSTALLER%" %*
set "HACO_INSTALL_EXIT=%ERRORLEVEL%"

:finish
if "%HACO_INSTALL_EXIT%"=="3010" (
    echo.
    echo Hacocoon installation is paused until Windows restarts. Follow the saved continuation instructions.
    goto wait_for_close
)

if not "%HACO_INSTALL_EXIT%"=="0" (
    echo.
    echo Hacocoon installation failed with exit code %HACO_INSTALL_EXIT%. 1>&2
    goto wait_for_close
)

echo.
echo Hacocoon Windows installation complete.

:wait_for_close
rem Automation can opt out without adding an argument to the PowerShell installer.
if "%HACO_INSTALL_NO_PAUSE%"=="1" goto return_result
if defined CI goto return_result
echo Press any key to close this installer window.
pause >nul

:return_result
exit /b %HACO_INSTALL_EXIT%
