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
title AntiDPI - Settings & Autostart
echo ===================================================================
echo                 AntiDPI - Settings
echo ===================================================================
echo.
echo   [1] Enable Windows Autostart (Start automatically on system logon)
echo   [2] Disable Windows Autostart (Remove from startup)
echo   [3] Reset & Fix Internet (Turn off proxy if closed unexpectedly)
echo   [4] Exit
echo.
echo ===================================================================
set /p choice="Select an option [1, 2, 3, 4]: "

if "%choice%"=="1" goto ADD
if "%choice%"=="2" goto REMOVE
if "%choice%"=="3" goto RESET
if "%choice%"=="4" exit /b
goto MENU

:ADD
cls
echo [*] Enabling Windows Autostart (Setting scheduled task with highest privileges)...
schtasks /create /tn "AntiDPI" /tr "\"%~dp0bin\antidpi.exe\"" /sc onlogon /rl highest /f >nul 2>&1
if %ERRORLEVEL% equ 0 (
    echo.
    echo [SUCCESS] AntiDPI has been added to Windows Autostart!
    echo It will automatically start every time your PC boots up.
) else (
    powershell -NoProfile -Command "$ws = New-Object -ComObject WScript.Shell; $s = $ws.CreateShortcut(\"$env:APPDATA\Microsoft\Windows\Start Menu\Programs\Startup\AntiDPI.lnk\"); $s.TargetPath = \"%~dp0bin\antidpi.exe\"; $s.WorkingDirectory = \"%~dp0bin\"; $s.Save()" >nul 2>&1
    echo.
    echo [SUCCESS] Added to Windows Startup folder.
)
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
cls
echo [*] Resetting Windows Proxy and DNS Settings to default...
for /f "tokens=4*" %%a in ('netsh interface show interface ^| findstr "Connected"') do netsh interface ip set dns name="%%b" dhcp >nul 2>&1
ipconfig /flushdns >nul 2>&1
reg add "HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings" /v ProxyEnable /t REG_DWORD /d 0 /f >nul 2>&1
reg add "HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings" /v ProxyServer /t REG_SZ /d "" /f >nul 2>&1
netsh advfirewall firewall delete rule name="AntiDPI_BlockQUIC" >nul 2>&1
powershell -NoProfile -Command "$s=@'[DllImport(\"wininet.dll\")]public static extern bool InternetSetOption(IntPtr h,int o,IntPtr b,int l);'@;$w=Add-Type -MemberDefinition $s -Name W -PassThru;[W]::InternetSetOption(0,39,0,0);[W]::InternetSetOption(0,37,0,0)" >nul 2>&1
echo.
echo [SUCCESS] Windows proxy has been completely disabled and refreshed.
echo Direct internet connection restored!
echo.
pause
goto MENU
