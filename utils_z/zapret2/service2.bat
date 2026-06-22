@echo off
setlocal EnableExtensions EnableDelayedExpansion
chcp 65001 >nul

set "SERVICE_NAME=zapret2"
set "BYPASS_EXE=winws2.exe"
set "ROOT=%~dp0"
set "BIN=%ROOT%bin"
set "UTILS=%ROOT%utils"

if /i "%~1"=="install" goto :install
if /i "%~1"=="remove" goto :remove
if /i "%~1"=="status" goto :status
if /i "%~1"=="tests" goto :tests
if /i "%~1"=="help" goto :help
if "%~1"=="" goto :menu

echo Unknown command: %~1
exit /b 1

:require_admin
net session >nul 2>&1 || (
    powershell -NoProfile -Command "Start-Process '%~f0' -ArgumentList '%*' -Verb RunAs"
    exit /b 1
)
exit /b 0

:menu
call :require_admin %*
cls
echo ================================
echo    ZAPRET2 SERVICE MANAGER
echo ================================
echo 1. Install service
echo 2. Remove service
echo 3. Status
echo 4. Run tests
echo 0. Exit
set /p "choice=Select: "
if "%choice%"=="1" goto :install
if "%choice%"=="2" goto :remove
if "%choice%"=="3" goto :status
if "%choice%"=="4" goto :tests
if "%choice%"=="0" exit /b 0
goto :menu

:install
call :require_admin %*
if not exist "%BIN%\%BYPASS_EXE%" (
    echo Missing %BYPASS_EXE% in %BIN%
    pause
    exit /b 1
)

set "CFG="
for /f "delims=" %%F in ('powershell -NoProfile -Command "Get-ChildItem -LiteralPath ''%ROOT%'' -Filter ''.bat'' ^| Where-Object { $_.Name -notlike ''service*'' } ^| Sort-Object Name ^| Select-Object -First 1 -ExpandProperty Name"') do set "CFG=%%F"
if not defined CFG (
    echo No config .bat found in %ROOT%
    pause
    exit /b 1
)

sc stop "%SERVICE_NAME%" >nul 2>&1
sc delete "%SERVICE_NAME%" >nul 2>&1
sc create "%SERVICE_NAME%" binPath= "\"%BIN%\%BYPASS_EXE%\" %CFG%" start= auto DisplayName= "Zapret2" >nul
sc description "%SERVICE_NAME%" "zapret2 service using winws2" >nul 2>&1
sc start "%SERVICE_NAME%" >nul 2>&1

echo Installed %SERVICE_NAME% with %CFG%
pause
exit /b 0

:remove
call :require_admin %*
for %%P in (winws2.exe winws.exe) do taskkill /IM %%P /F >nul 2>&1
sc stop "%SERVICE_NAME%" >nul 2>&1
sc delete "%SERVICE_NAME%" >nul 2>&1
echo Removed %SERVICE_NAME%
pause
exit /b 0

:status
sc query "%SERVICE_NAME%"
for %%P in (winws2.exe winws.exe) do (
    tasklist /FI "IMAGENAME eq %%P" | find /I "%%P" >nul && echo Running: %%P
)
pause
exit /b 0

:tests
if exist "%UTILS%\test_zapret2.ps1" (
    powershell -NoProfile -ExecutionPolicy Bypass -File "%UTILS%\test_zapret2.ps1"
) else (
    echo Missing %UTILS%\test_zapret2.ps1
)
pause
exit /b 0

:help
echo install  - install zapret2 service
echo remove   - remove zapret2 service
echo status   - show service status
echo tests    - run test_zapret2.ps1
exit /b 0
