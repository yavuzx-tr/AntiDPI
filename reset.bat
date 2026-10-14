@echo off
setlocal
cd /d "%~dp0"

:: Request administrator privileges
net session >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo [!] Requesting administrator privileges...
    powershell -NoProfile -ExecutionPolicy Bypass -Command "Start-Process -FilePath '%~f0' -Verb RunAs"
    exit /b
)

title AntiDPI - Network & Settings Recovery Tool
cls

echo ===================================================================
echo               AntiDPI - Network Recovery Tool
echo ===================================================================
echo.
echo [*] Terminating active AntiDPI processes...
taskkill /f /im antidpi.exe >nul 2>&1
taskkill /f /im antidpi.old.exe >nul 2>&1

echo [*] Resetting Windows Proxy settings...
reg add "HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings" /v ProxyEnable /t REG_DWORD /d 0 /f >nul 2>&1
reg delete "HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings" /v ProxyServer /f >nul 2>&1
reg delete "HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings" /v AutoConfigURL /f >nul 2>&1
netsh winhttp reset proxy >nul 2>&1

echo [*] Clearing environment variables...
reg delete "HKCU\Environment" /v HTTP_PROXY /f >nul 2>&1
reg delete "HKCU\Environment" /v HTTPS_PROXY /f >nul 2>&1
reg delete "HKCU\Environment" /v ALL_PROXY /f >nul 2>&1
reg delete "HKCU\Environment" /v NO_PROXY /f >nul 2>&1

echo [*] Restoring DNS to automatic (DHCP)...
powershell -NoProfile -ExecutionPolicy Bypass -Command "Get-NetAdapter | ForEach-Object { Set-DnsClientServerAddress -InterfaceIndex $_.InterfaceIndex -ResetServerAddresses -ErrorAction SilentlyContinue }; Enable-NetAdapterBinding -Name * -ComponentId ms_tcpip6 -ErrorAction SilentlyContinue" >nul 2>&1
netsh interface ip set dns name="Wi-Fi" dhcp >nul 2>&1
netsh interface ip set dns name="Ethernet" dhcp >nul 2>&1

echo [*] Cleaning AntiDPI entries from Windows Hosts file...
powershell -NoProfile -ExecutionPolicy Bypass -Command "$p = '$env:WINDIR\System32\drivers\etc\hosts'; if (Test-Path $p) { $c = [System.IO.File]::ReadAllText($p); $c = [System.Text.RegularExpressions.Regex]::Replace($c, '(?s)# --- ANTIDPI.*?# --- ANTIDPI.*?END ---[\r\n]*', ''); [System.IO.File]::WriteAllText($p, $c) }" >nul 2>&1

echo [*] Removing firewall rules...
netsh advfirewall firewall delete rule name=AntiDPI_BlockQUIC >nul 2>&1

echo [*] Flushing DNS cache...
ipconfig /flushdns >nul 2>&1

echo.
echo ===================================================================
echo [OK] Network settings restored to defaults.
echo      All connections should function normally.
echo ===================================================================
echo.
pause
