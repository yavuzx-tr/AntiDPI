@echo off
setlocal
cd /d "%~dp0"

net session >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo [!] Requesting administrator privileges...
    powershell -NoProfile -ExecutionPolicy Bypass -Command "Start-Process -FilePath '%~f0' -Verb RunAs"
    exit /b
)

title AntiDPI
cls
"%~dp0bin\antidpi.exe" -dns

:: Fail-safe cleanup
for /f "tokens=4*" %%a in ('netsh interface show interface ^| findstr "Connected"') do netsh interface ip set dns name="%%b" dhcp >nul 2>&1
ipconfig /flushdns >nul 2>&1
reg add "HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings" /v ProxyEnable /t REG_DWORD /d 0 /f >nul 2>&1
reg add "HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings" /v ProxyServer /t REG_SZ /d "" /f >nul 2>&1
netsh advfirewall firewall delete rule name="AntiDPI_BlockQUIC" >nul 2>&1
powershell -NoProfile -Command "$s=@'[DllImport(\"wininet.dll\")]public static extern bool InternetSetOption(IntPtr h,int o,IntPtr b,int l);'@;$w=Add-Type -MemberDefinition $s -Name W -PassThru;[W]::InternetSetOption(0,39,0,0);[W]::InternetSetOption(0,37,0,0)" >nul 2>&1
