@echo off
setlocal
cd /d "%~dp0"

:: Request administrator privileges
net session >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo [!] Yonetici yetkileri isteniyor...
    powershell -NoProfile -ExecutionPolicy Bypass -Command "Start-Process -FilePath '%~f0' -Verb RunAs"
    exit /b
)

title AntiDPI - Ag ve Internet Ayarlarini Sifirla
cls

echo ===================================================================
echo               AntiDPI - Internet ve Ag Kurtarma Araci
echo ===================================================================
echo.
echo [*] Calisan AntiDPI islemleri durduruluyor...
taskkill /f /im antidpi.exe >nul 2>&1
taskkill /f /im antidpi.old.exe >nul 2>&1

echo [*] Windows Proxy ayarlari sifirlaniyor...
reg add "HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings" /v ProxyEnable /t REG_DWORD /d 0 /f >nul 2>&1
reg delete "HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings" /v ProxyServer /f >nul 2>&1
reg delete "HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings" /v AutoConfigURL /f >nul 2>&1
netsh winhttp reset proxy >nul 2>&1

echo [*] Ortam degiskenleri temizleniyor...
reg delete "HKCU\Environment" /v HTTP_PROXY /f >nul 2>&1
reg delete "HKCU\Environment" /v HTTPS_PROXY /f >nul 2>&1
reg delete "HKCU\Environment" /v ALL_PROXY /f >nul 2>&1
reg delete "HKCU\Environment" /v NO_PROXY /f >nul 2>&1

echo [*] Ag bagdastirici DNS ayarlari otomatie (DHCP) aliniyor...
powershell -NoProfile -ExecutionPolicy Bypass -Command "Get-NetAdapter | ForEach-Object { Set-DnsClientServerAddress -InterfaceIndex $_.InterfaceIndex -ResetServerAddresses -ErrorAction SilentlyContinue }; Enable-NetAdapterBinding -Name * -ComponentId ms_tcpip6 -ErrorAction SilentlyContinue" >nul 2>&1
netsh interface ip set dns name="Wi-Fi" dhcp >nul 2>&1
netsh interface ip set dns name="Ethernet" dhcp >nul 2>&1

echo [*] Windows Hosts dosyasindaki AntiDPI kayitlari temizleniyor...
powershell -NoProfile -ExecutionPolicy Bypass -Command "$p = '$env:WINDIR\System32\drivers\etc\hosts'; if (Test-Path $p) { $c = [System.IO.File]::ReadAllText($p); $c = [System.Text.RegularExpressions.Regex]::Replace($c, '(?s)# --- ANTIDPI.*?# --- ANTIDPI.*?END ---[\r\n]*', ''); [System.IO.File]::WriteAllText($p, $c) }" >nul 2>&1

echo [*] Guvenlik duvari kurallari temizleniyor...
netsh advfirewall firewall delete rule name=AntiDPI_BlockQUIC >nul 2>&1

echo [*] DNS onbellegi temizleniyor...
ipconfig /flushdns >nul 2>&1

echo.
echo ===================================================================
echo [OK] Internet ve ag ayarlari tamamen fabrika ayarlarina donduruldu!
echo      Tum baglantiniz sorunsuz calisacaktir.
echo ===================================================================
echo.
pause
