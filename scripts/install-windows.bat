@echo off
setlocal

set "INSTALLER=%~dp0install-windows.ps1"
set "WINDOWS_LAUNCHER=%~dp0haco-windows.ps1"

where powershell.exe >nul 2>nul
if errorlevel 1 (
    echo Hacocoon installer error: powershell.exe was not found. 1>&2
    exit /b 1
)

if not exist "%INSTALLER%" (
    echo Hacocoon installer error: install-windows.ps1 must be next to install-windows.bat. 1>&2
    exit /b 1
)
if not exist "%WINDOWS_LAUNCHER%" (
    echo Hacocoon installer error: haco-windows.ps1 must be next to install-windows.bat. 1>&2
    exit /b 1
)

powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%INSTALLER%" %*
set "HACO_INSTALL_EXIT=%ERRORLEVEL%"

if not "%HACO_INSTALL_EXIT%"=="0" (
    echo.
    echo Hacocoon installation failed with exit code %HACO_INSTALL_EXIT%. 1>&2
    exit /b %HACO_INSTALL_EXIT%
)

set "HACO_INSTANCE=Hacocoon"
call :resolve_instance %*
powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%WINDOWS_LAUNCHER%" __install-launcher "%HACO_INSTANCE%"
set "HACO_LAUNCHER_EXIT=%ERRORLEVEL%"
if not "%HACO_LAUNCHER_EXIT%"=="0" (
    echo.
    echo Hacocoon Windows launcher setup failed with exit code %HACO_LAUNCHER_EXIT%. 1>&2
    exit /b %HACO_LAUNCHER_EXIT%
)

echo.
echo Hacocoon Windows installation complete.
exit /b 0

:resolve_instance
if "%~1"=="" exit /b 0
if /I "%~1"=="-InstanceName" (
    if "%~2"=="" exit /b 2
    set "HACO_INSTANCE=%~2"
    exit /b 0
)
shift
goto resolve_instance
