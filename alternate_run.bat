@echo off
setlocal
cd /d "%~dp0"

net session >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo [!] Requesting administrator privileges...
    powershell -NoProfile -ExecutionPolicy Bypass -Command "Start-Process -FilePath '%~f0' -Verb RunAs"
    exit /b
)

title AntiDPI (Extended DNS Mode)
cls

:: Disable IPv6 to prevent ISP fake AAAA sinkhole poisoning
powershell -NoProfile -ExecutionPolicy Bypass -Command "Disable-NetAdapterBinding -Name * -ComponentId ms_tcpip6 -ErrorAction SilentlyContinue" >nul 2>&1

:: Configure adapter DNS to 127.0.0.1 DoH relay
powershell -NoProfile -ExecutionPolicy Bypass -Command "Get-NetAdapter | Where-Object { $_.Status -eq 'Up' -and $_.InterfaceDescription -notlike '*Virtual*' } | ForEach-Object { Set-DnsClientServerAddress -InterfaceIndex $_.InterfaceIndex -ServerAddresses '127.0.0.1' }" >nul 2>&1
netsh interface ip set dns name="Wi-Fi" static 127.0.0.1 >nul 2>&1
netsh interface ip set dns name="Ethernet" static 127.0.0.1 >nul 2>&1
ipconfig /flushdns >nul 2>&1

"%~dp0bin\antidpi.exe" -dns

:: Fail-safe cleanup
powershell -NoProfile -ExecutionPolicy Bypass -Command "Get-NetAdapter | Where-Object { $_.Status -eq 'Up' -and $_.InterfaceDescription -notlike '*Virtual*' } | ForEach-Object { Set-DnsClientServerAddress -InterfaceIndex $_.InterfaceIndex -ResetServerAddresses }" >nul 2>&1
netsh interface ip set dns name="Wi-Fi" dhcp >nul 2>&1
netsh interface ip set dns name="Ethernet" dhcp >nul 2>&1
powershell -NoProfile -ExecutionPolicy Bypass -Command "Enable-NetAdapterBinding -Name * -ComponentId ms_tcpip6 -ErrorAction SilentlyContinue" >nul 2>&1
powershell -NoProfile -ExecutionPolicy Bypass -Command "$h = 'C:\Windows\System32\drivers\etc\hosts'; if (Test-Path $h) { $c = [IO.File]::ReadAllText($h); $c = [Text.RegularExpressions.Regex]::Replace($c, '(?s)# --- ANTIDPI ROBLOX BYPASS START ---.*?# --- ANTIDPI ROBLOX BYPASS END ---\r?\n?', ''); [IO.File]::WriteAllText($h, $c) }" >nul 2>&1
ipconfig /flushdns >nul 2>&1
reg add "HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings" /v ProxyEnable /t REG_DWORD /d 0 /f >nul 2>&1
reg add "HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings" /v ProxyServer /t REG_SZ /d "" /f >nul 2>&1
netsh advfirewall firewall delete rule name="AntiDPI_BlockQUIC" >nul 2>&1
powershell -NoProfile -Command "$s=@'[DllImport(\"wininet.dll\")]public static extern bool InternetSetOption(IntPtr h,int o,IntPtr b,int l);'@;$w=Add-Type -MemberDefinition $s -Name W -PassThru;[W]::InternetSetOption(0,39,0,0);[W]::InternetSetOption(0,37,0,0)" >nul 2>&1
