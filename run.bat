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

title AntiDPI 2.0 - Standalone TCP Desync & Bypass Engine
cls

:: Grant write permissions to hosts file
icacls "%WINDIR%\System32\drivers\etc\hosts" /grant "Users:(M)" /c /q >nul 2>&1

:: Terminate any lingering instances
taskkill /f /im antidpi.exe >nul 2>&1
taskkill /f /im antidpi.old.exe >nul 2>&1
if exist "%~dp0bin\antidpi.new.exe" (
    move /y "%~dp0bin\antidpi.new.exe" "%~dp0bin\antidpi.exe" >nul 2>&1
)
if exist "%~dp0bin\antidpi.old.exe" del /f /q "%~dp0bin\antidpi.old.exe" >nul 2>&1

:: Enable proxy environment variables for game engines and libcurl
powershell -NoProfile -ExecutionPolicy Bypass -Command "[System.Environment]::SetEnvironmentVariable('ALL_PROXY', 'http://127.0.0.1:8080', 'User'); [System.Environment]::SetEnvironmentVariable('HTTPS_PROXY', 'http://127.0.0.1:8080', 'User'); [System.Environment]::SetEnvironmentVariable('HTTP_PROXY', 'http://127.0.0.1:8080', 'User'); [System.Environment]::SetEnvironmentVariable('NO_PROXY', 'localhost,127.0.0.1', 'User')" >nul 2>&1
set "ALL_PROXY=http://127.0.0.1:8080"
set "HTTPS_PROXY=http://127.0.0.1:8080"
set "HTTP_PROXY=http://127.0.0.1:8080"
set "NO_PROXY=localhost,127.0.0.1"

:: Disable IPv6 sinkholing on network adapters
powershell -NoProfile -ExecutionPolicy Bypass -Command "Disable-NetAdapterBinding -Name * -ComponentId ms_tcpip6 -ErrorAction SilentlyContinue" >nul 2>&1

:: Flush DNS cache
ipconfig /flushdns >nul 2>&1

cls
echo.
echo   [/] Starting AntiDPI 2.0 Engine...
echo.

:: Launch pure Go standalone engine
"%~dp0bin\antidpi.exe" -autoproxy=true -dns=true

:: Cleanup on exit
cls
echo.
echo [*] Restoring network settings...
powershell -NoProfile -ExecutionPolicy Bypass -Command "[System.Environment]::SetEnvironmentVariable('ALL_PROXY', $null, 'User'); [System.Environment]::SetEnvironmentVariable('HTTPS_PROXY', $null, 'User'); [System.Environment]::SetEnvironmentVariable('HTTP_PROXY', $null, 'User'); [System.Environment]::SetEnvironmentVariable('NO_PROXY', $null, 'User')" >nul 2>&1
powershell -NoProfile -ExecutionPolicy Bypass -Command "Get-NetAdapter | Where-Object { $_.Status -eq 'Up' -and $_.InterfaceDescription -notlike '*Virtual*' } | ForEach-Object { Set-DnsClientServerAddress -InterfaceIndex $_.InterfaceIndex -ResetServerAddresses -ErrorAction SilentlyContinue }" >nul 2>&1
powershell -NoProfile -ExecutionPolicy Bypass -Command "Enable-NetAdapterBinding -Name * -ComponentId ms_tcpip6 -ErrorAction SilentlyContinue" >nul 2>&1
reg add "HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings" /v ProxyEnable /t REG_DWORD /d 0 /f >nul 2>&1
reg add "HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings" /v ProxyServer /t REG_SZ /d "" /f >nul 2>&1
ipconfig /flushdns >nul 2>&1
timeout /t 1 /nobreak >nul
