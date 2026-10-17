@echo off
setlocal
cd /d "%~dp0"

:: Check Administrator rights
net session >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo [!] Requesting administrator privileges...
    powershell -NoProfile -ExecutionPolicy Bypass -Command "Start-Process -FilePath '%~f0' -Verb RunAs"
    exit /b
)

:MENU
cls
title AntiDPI - Settings & Autostart (Windows)
echo ===================================================================
echo               AntiDPI - Settings & Autostart (Windows)
echo ===================================================================
echo.
echo   [1] Enable Windows Silent Autostart (Starts hidden on boot, no window)
echo   [2] Start AntiDPI in Background Now (Silent, no console window)
echo   [3] Disable Windows Autostart (Remove from startup)
echo   [4] Stop AntiDPI ^& Reset Network (Clean exit ^& direct internet)
echo   [5] Exit
echo.
echo   * Note for PM2 users: You can also run 'pm2 start ecosystem.config.js'
echo ===================================================================
set /p choice="Select an option [1-5]: "

if "%choice%"=="1" goto ADD
if "%choice%"=="2" goto RUN_BACKGROUND
if "%choice%"=="3" goto REMOVE
if "%choice%"=="4" goto RESET
if "%choice%"=="5" exit /b
goto MENU

:ADD
cls
echo [*] Enabling Windows Silent Autostart (Setting scheduled task with highest privileges)...
schtasks /create /tn "AntiDPI" /tr "wscript.exe \"%~dp0run_silent.vbs\"" /sc onlogon /rl highest /f >nul 2>&1
if %ERRORLEVEL% equ 0 (
    echo.
    echo [SUCCESS] AntiDPI has been added to Windows Autostart (Silent Mode)!
    echo It will start automatically and silently in the background on system boot.
) else (
    powershell -NoProfile -Command "$ws = New-Object -ComObject WScript.Shell; $s = $ws.CreateShortcut(\"$env:APPDATA\Microsoft\Windows\Start Menu\Programs\Startup\AntiDPI.lnk\"); $s.TargetPath = \"wscript.exe\"; $s.Arguments = \"`\"%~dp0run_silent.vbs`\"\"; $s.WorkingDirectory = \"%~dp0\"; $s.Save()" >nul 2>&1
    echo.
    echo [SUCCESS] Added to Windows Startup folder (Silent Mode).
)
echo.
pause
goto MENU

:RUN_BACKGROUND
cls
echo [*] Starting AntiDPI in background (Silent Mode)...
start "" wscript.exe "%~dp0run_silent.vbs"
timeout /t 2 /nobreak >nul
echo.
echo [SUCCESS] AntiDPI is now running in the background.
echo Direct internet protection active (Roblox ^& Discord unblocked).
echo.
pause
goto MENU

:REMOVE
cls
echo [*] Removing from Windows Autostart...
schtasks /delete /tn "AntiDPI" /f >nul 2>&1
if exist "%APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup\AntiDPI.lnk" (
    del /f /q "%APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup\AntiDPI.lnk" >nul 2>&1
)
echo.
echo [OK] AntiDPI has been removed from Windows Autostart.
echo.
pause
goto MENU

:RESET
call "%~dp0reset.bat"
goto MENU
