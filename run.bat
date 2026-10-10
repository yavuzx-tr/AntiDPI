@echo off
setlocal
cd /d "%~dp0"

:: Yonetici haklari kontrolu
net session >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo [!] Yonetici yetkileri isteniyor...
    powershell -NoProfile -ExecutionPolicy Bypass -Command "Start-Process -FilePath '%~f0' -Verb RunAs"
    exit /b
)

title AntiDPI (WinDivert Kernel Motoru)
cls

echo ===================================================================
echo               AntiDPI - WinDivert Kernel Motoru
echo         Roblox, Discord ve Tum Yasakli Siteler Icin %%100 Cozum
echo ===================================================================
echo.
echo [*] Ag yapilandirmasi hazirlaniyor...

:: Onceki takili kalmis proxy veya vekilleri tamamen kaldir
reg add "HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings" /v ProxyEnable /t REG_DWORD /d 0 /f >nul 2>&1
reg add "HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings" /v ProxyServer /t REG_SZ /d "" /f >nul 2>&1
REG DELETE "HKCU\Environment" /v ALL_PROXY /f 2>$null
REG DELETE "HKCU\Environment" /v all_proxy /f 2>$null
REG DELETE "HKCU\Environment" /v HTTP_PROXY /f 2>$null
REG DELETE "HKCU\Environment" /v HTTPS_PROXY /f 2>$null

:: Turk Telekom sahte IPv6 AAAA tuzagini devre disi birak
powershell -NoProfile -ExecutionPolicy Bypass -Command "Disable-NetAdapterBinding -Name * -ComponentId ms_tcpip6 -ErrorAction SilentlyContinue" >nul 2>&1

:: Roblox dogrudan IP erisimini saglamak icin hosts kayitlarini uygula
powershell -NoProfile -ExecutionPolicy Bypass -Command "$h = 'C:\Windows\System32\drivers\etc\hosts'; if (Test-Path $h) { $c = [IO.File]::ReadAllText($h); $c = [Text.RegularExpressions.Regex]::Replace($c, '(?s)# --- ANTIDPI ROBLOX BYPASS START ---.*?# --- ANTIDPI ROBLOX BYPASS END ---\r?\n?', ''); $b = \"`r`n# --- ANTIDPI ROBLOX BYPASS START ---`r`n128.116.5.3 roblox.com`r`n128.116.5.3 www.roblox.com`r`n128.116.31.3 apis.roblox.com`r`n128.116.31.3 ecsv2.roblox.com`r`n128.116.31.3 realtime-signalr.roblox.com`r`n128.116.31.3 notifications.roblox.com`r`n99.84.91.31 clientsettingscdn.roblox.com`r`n99.84.91.31 clientsettings.roblox.com`r`n128.116.5.3 gamejoin.roblox.com`r`n128.116.31.3 assetgame.roblox.com`r`n128.116.5.3 versioncompatibility.api.roblox.com`r`n2.16.16.154 setup.rbxcdn.com`r`n# --- ANTIDPI ROBLOX BYPASS END ---`r`n\"; [IO.File]::WriteAllText($h, ($c.TrimEnd() + $b)) }" >nul 2>&1

:: DNS onbellegini temizle
ipconfig /flushdns >nul 2>&1

echo [OK] Ag ayarlari tamamlandi.
echo [*] Kernel seviyesinde WinDivert motoru calistiriliyor...
echo.
echo -------------------------------------------------------------------
echo  KORUMA AKTIF: Roblox ve Discord artik sorunsuz calisacak!
echo  Kapatmak icin bu pencereyi kapatin veya Ctrl+C yapin.
echo -------------------------------------------------------------------
echo.

:: WinDivert tabanli motoru Turkiye modu ile calistir
"%~dp0bin\goodbyedpi.exe" -5 --dns-addr 77.88.8.8 --dns-port 1253 --dnsv6-addr 2a02:6b8::feed:0ff --dnsv6-port 1253

:: Cikis ve temizlik
echo.
echo [*] Temizlik yapiliyor...
powershell -NoProfile -ExecutionPolicy Bypass -Command "Enable-NetAdapterBinding -Name * -ComponentId ms_tcpip6 -ErrorAction SilentlyContinue" >nul 2>&1
powershell -NoProfile -ExecutionPolicy Bypass -Command "$h = 'C:\Windows\System32\drivers\etc\hosts'; if (Test-Path $h) { $c = [IO.File]::ReadAllText($h); $c = [Text.RegularExpressions.Regex]::Replace($c, '(?s)# --- ANTIDPI ROBLOX BYPASS START ---.*?# --- ANTIDPI ROBLOX BYPASS END ---\r?\n?', ''); [IO.File]::WriteAllText($h, $c) }" >nul 2>&1
ipconfig /flushdns >nul 2>&1
echo [OK] Internet ayarlari normale donduruldu.
timeout /t 2 /nobreak >nul
