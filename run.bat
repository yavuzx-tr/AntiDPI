@echo off
setlocal
cd /d "%~dp0"

:: Check administrator privileges
net session >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo [!] Requesting administrator privileges...
    powershell -NoProfile -ExecutionPolicy Bypass -Command "Start-Process -FilePath '%~f0' -Verb RunAs"
    exit /b
)

title AntiDPI - Standalone TCP Desync & Bypass Engine
cls

:: Grant write permissions to hosts file
icacls "%WINDIR%\System32\drivers\etc\hosts" /grant "Users:(M)" /c /q >nul 2>&1

:: Clean any stale environment variables from previous versions
reg delete "HKCU\Environment" /v HTTP_PROXY /f >nul 2>&1
reg delete "HKCU\Environment" /v HTTPS_PROXY /f >nul 2>&1
reg delete "HKCU\Environment" /v ALL_PROXY /f >nul 2>&1
reg delete "HKCU\Environment" /v NO_PROXY /f >nul 2>&1

:: Terminate any lingering instances of antidpi and game/voice clients
taskkill /f /im antidpi.exe >nul 2>&1
taskkill /f /im antidpi.old.exe >nul 2>&1
taskkill /f /im RobloxPlayerBeta.exe >nul 2>&1
taskkill /f /im RobloxCrashHandler.exe >nul 2>&1
taskkill /f /im Discord.exe >nul 2>&1
taskkill /f /im DiscordCanary.exe >nul 2>&1
taskkill /f /im DiscordPTB.exe >nul 2>&1
taskkill /f /im Update.exe >nul 2>&1
if exist "%~dp0bin\antidpi.new.exe" (
    move /y "%~dp0bin\antidpi.new.exe" "%~dp0bin\antidpi.exe" >nul 2>&1
)
if exist "%~dp0bin\antidpi.old.exe" del /f /q "%~dp0bin\antidpi.old.exe" >nul 2>&1

:: Flush DNS cache
ipconfig /flushdns >nul 2>&1

cls
echo.
echo   [/] Starting AntiDPI Engine (Zero-Risk Transparent Mode)...
echo.

:: Launch pure Go standalone engine (Surgical transparent routing - No global proxy, no adapter hijack)
"%~dp0bin\antidpi.exe" -pac=true -autoproxy=false -dns=false

:: Cleanup on exit
cls
echo.
echo [*] Restoring network settings...
powershell -NoProfile -ExecutionPolicy Bypass -Command "$p = '$env:WINDIR\System32\drivers\etc\hosts'; if (Test-Path $p) { $c = [System.IO.File]::ReadAllText($p); $c = [System.Text.RegularExpressions.Regex]::Replace($c, '(?s)# --- ANTIDPI.*?# --- ANTIDPI.*?END ---[\r\n]*', ''); [System.IO.File]::WriteAllText($p, $c) }" >nul 2>&1
reg delete "HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings" /v AutoConfigURL /f >nul 2>&1
netsh advfirewall firewall delete rule name=AntiDPI_BlockQUIC >nul 2>&1
ipconfig /flushdns >nul 2>&1
timeout /t 1 /nobreak >nul
