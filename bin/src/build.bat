@echo off
setlocal
cd /d "%~dp0"

echo ===================================================
echo [AntiDPI] Standalone Binary Compiler
echo ===================================================

where go >nul 2>&1
if %ERRORLEVEL% equ 0 (
    set "GOCMD=go"
) else (
    if exist "%USERPROFILE%\.gemini\antigravity\scratch\gotools\go\bin\go.exe" (
        set "GOROOT=%USERPROFILE%\.gemini\antigravity\scratch\gotools\go"
        set "GOCMD=%USERPROFILE%\.gemini\antigravity\scratch\gotools\go\bin\go.exe"
    ) else (
        echo [ERROR] Go compiler not found!
        pause
        exit /b 1
    )
)

echo [*] Building ..\antidpi.exe (Embedded icon, standalone static binary)...
set CGO_ENABLED=0
"%GOCMD%" build -trimpath -ldflags "-s -w" -o ..\antidpi.exe .

if %ERRORLEVEL% equ 0 (
    echo [OK] Build successful: ..\antidpi.exe
    dir ..\antidpi.exe | findstr "antidpi.exe"
) else (
    echo [ERROR] Build failed.
)

pause
