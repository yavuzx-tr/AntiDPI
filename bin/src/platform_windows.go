//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"syscall"
	"unsafe"
)

var (
	modAdvapi32              = syscall.NewLazyDLL("advapi32.dll")
	procRegOpenKeyExW        = modAdvapi32.NewProc("RegOpenKeyExW")
	procRegSetValueExW       = modAdvapi32.NewProc("RegSetValueExW")
	procRegDeleteValueW      = modAdvapi32.NewProc("RegDeleteValueW")
	procRegQueryValueExW     = modAdvapi32.NewProc("RegQueryValueExW")
	procRegCloseKey          = modAdvapi32.NewProc("RegCloseKey")
	procCheckTokenMembership = modAdvapi32.NewProc("CheckTokenMembership")
	modDnsapi                = syscall.NewLazyDLL("dnsapi.dll")
	procDnsFlushCache        = modDnsapi.NewProc("DnsFlushResolverCache")

	kernel32Dll               = syscall.NewLazyDLL("kernel32.dll")
	procSetConsoleCtrlHandler = kernel32Dll.NewProc("SetConsoleCtrlHandler")
	procGetStdHandle          = kernel32Dll.NewProc("GetStdHandle")
	procSetConsoleMode        = kernel32Dll.NewProc("SetConsoleMode")
	procGetConsoleMode        = kernel32Dll.NewProc("GetConsoleMode")
)

func initConsole() {
	const stdOutputHandle = ^uintptr(10) // -11
	const enableVirtualTerminalProcessing = 0x0004

	hOut, _, _ := procGetStdHandle.Call(stdOutputHandle)
	if hOut != 0 && hOut != ^uintptr(0) {
		var mode uint32
		r, _, _ := procGetConsoleMode.Call(hOut, uintptr(unsafe.Pointer(&mode)))
		if r != 0 {
			procSetConsoleMode.Call(hOut, uintptr(mode|enableVirtualTerminalProcessing))
		}
	}
}

func registerPlatformCloseHandler(cleanup func()) {
	cb := syscall.NewCallback(func(ctrlType uint32) uintptr {
		cleanup()
		return 1
	})
	procSetConsoleCtrlHandler.Call(cb, 1)
}

func getHostsPath() string {
	return `C:\Windows\System32\drivers\etc\hosts`
}

func FlushDNS() {
	procDnsFlushCache.Call()
}

func enableQUICBlock() {
	cmd := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		"name=AntiDPI_BlockQUIC", "dir=out", "action=block", "protocol=UDP", "remoteport=443")
	_ = cmd.Run()
}

func disableQUICBlock() {
	cmd := exec.Command("netsh", "advfirewall", "firewall", "delete", "rule", "name=AntiDPI_BlockQUIC")
	_ = cmd.Run()
}

func IsAdmin() bool {
	var adminSID *byte
	ntAuth := [6]byte{0, 0, 0, 0, 0, 5}
	r, _, err := syscall.NewLazyDLL("advapi32.dll").NewProc("AllocateAndInitializeSid").Call(
		uintptr(unsafe.Pointer(&ntAuth[0])),
		2,
		uintptr(32),
		uintptr(544),
		0, 0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&adminSID)),
	)
	if r == 0 || adminSID == nil {
		_ = err
		return false
	}
	defer syscall.NewLazyDLL("advapi32.dll").NewProc("FreeSid").Call(uintptr(unsafe.Pointer(adminSID)))

	var isMember int32
	r, _, _ = procCheckTokenMembership.Call(0, uintptr(unsafe.Pointer(adminSID)), uintptr(unsafe.Pointer(&isMember)))
	return r != 0 && isMember != 0
}

const (
	HKEY_CURRENT_USER                = 0x80000001
	KEY_READ                         = 0x20019
	KEY_WRITE                        = 0x20006
	REG_DWORD                        = 4
	REG_SZ                           = 1
	INTERNET_OPTION_SETTINGS_CHANGED = 39
	INTERNET_OPTION_REFRESH          = 37
)

type SavedProxySettings struct {
	Enabled       uint32
	Server        string
	Override      string
	WasConfigured bool
}

var initialSettings SavedProxySettings

func EnableSystemProxy(proxyAddr string) error {
	saveCurrentProxySettings()

	subKey, _ := syscall.UTF16PtrFromString(`Software\Microsoft\Windows\CurrentVersion\Internet Settings`)
	var hKey uintptr
	r, _, _ := procRegOpenKeyExW.Call(
		uintptr(HKEY_CURRENT_USER),
		uintptr(unsafe.Pointer(subKey)),
		0,
		uintptr(KEY_WRITE),
		uintptr(unsafe.Pointer(&hKey)),
	)
	if r != 0 {
		return fmt.Errorf("failed to open registry key: %d", r)
	}
	defer procRegCloseKey.Call(hKey)

	enableVal := uint32(1)
	valName, _ := syscall.UTF16PtrFromString("ProxyEnable")
	procRegSetValueExW.Call(
		hKey,
		uintptr(unsafe.Pointer(valName)),
		0,
		uintptr(REG_DWORD),
		uintptr(unsafe.Pointer(&enableVal)),
		uintptr(4),
	)

	serverVal, _ := syscall.UTF16FromString(proxyAddr)
	serverName, _ := syscall.UTF16PtrFromString("ProxyServer")
	procRegSetValueExW.Call(
		hKey,
		uintptr(unsafe.Pointer(serverName)),
		0,
		uintptr(REG_SZ),
		uintptr(unsafe.Pointer(&serverVal[0])),
		uintptr(len(serverVal)*2),
	)

	overrideVal, _ := syscall.UTF16FromString("<local>;localhost;127.0.0.1")
	overrideName, _ := syscall.UTF16PtrFromString("ProxyOverride")
	procRegSetValueExW.Call(
		hKey,
		uintptr(unsafe.Pointer(overrideName)),
		0,
		uintptr(REG_SZ),
		uintptr(unsafe.Pointer(&overrideVal[0])),
		uintptr(len(overrideVal)*2),
	)

	notifyWinINet()
	enableEnvProxy("http://" + proxyAddr)
	return nil
}

func DisableSystemProxy() {
	subKey, _ := syscall.UTF16PtrFromString(`Software\Microsoft\Windows\CurrentVersion\Internet Settings`)
	var hKey uintptr
	r, _, _ := procRegOpenKeyExW.Call(
		uintptr(HKEY_CURRENT_USER),
		uintptr(unsafe.Pointer(subKey)),
		0,
		uintptr(KEY_WRITE),
		uintptr(unsafe.Pointer(&hKey)),
	)
	if r != 0 {
		return
	}
	defer procRegCloseKey.Call(hKey)

	zeroVal := uint32(0)
	valName, _ := syscall.UTF16PtrFromString("ProxyEnable")
	procRegSetValueExW.Call(
		hKey,
		uintptr(unsafe.Pointer(valName)),
		0,
		uintptr(REG_DWORD),
		uintptr(unsafe.Pointer(&zeroVal)),
		uintptr(4),
	)

	emptyVal, _ := syscall.UTF16FromString("")
	serverName, _ := syscall.UTF16PtrFromString("ProxyServer")
	procRegSetValueExW.Call(
		hKey,
		uintptr(unsafe.Pointer(serverName)),
		0,
		uintptr(REG_SZ),
		uintptr(unsafe.Pointer(&emptyVal[0])),
		uintptr(len(emptyVal)*2),
	)

	notifyWinINet()
	disableEnvProxy()
}

func EnablePAC(pacURL string) error {
	subKey, _ := syscall.UTF16PtrFromString(`Software\Microsoft\Windows\CurrentVersion\Internet Settings`)
	var hKey uintptr
	r, _, _ := procRegOpenKeyExW.Call(
		uintptr(HKEY_CURRENT_USER),
		uintptr(unsafe.Pointer(subKey)),
		0,
		uintptr(KEY_WRITE),
		uintptr(unsafe.Pointer(&hKey)),
	)
	if r != 0 {
		return fmt.Errorf("failed to open registry key: %d", r)
	}
	defer procRegCloseKey.Call(hKey)

	val, _ := syscall.UTF16FromString(pacURL)
	valName, _ := syscall.UTF16PtrFromString("AutoConfigURL")
	procRegSetValueExW.Call(
		hKey,
		uintptr(unsafe.Pointer(valName)),
		0,
		uintptr(REG_SZ),
		uintptr(unsafe.Pointer(&val[0])),
		uintptr(len(val)*2),
	)

	zeroVal := uint32(0)
	peName, _ := syscall.UTF16PtrFromString("ProxyEnable")
	procRegSetValueExW.Call(
		hKey,
		uintptr(unsafe.Pointer(peName)),
		0,
		uintptr(REG_DWORD),
		uintptr(unsafe.Pointer(&zeroVal)),
		uintptr(4),
	)

	notifyWinINet()
	return nil
}

func DisablePAC() {
	subKey, _ := syscall.UTF16PtrFromString(`Software\Microsoft\Windows\CurrentVersion\Internet Settings`)
	var hKey uintptr
	r, _, _ := procRegOpenKeyExW.Call(
		uintptr(HKEY_CURRENT_USER),
		uintptr(unsafe.Pointer(subKey)),
		0,
		uintptr(KEY_WRITE),
		uintptr(unsafe.Pointer(&hKey)),
	)
	if r != 0 {
		return
	}
	defer procRegCloseKey.Call(hKey)

	valName, _ := syscall.UTF16PtrFromString("AutoConfigURL")
	procRegDeleteValueW.Call(hKey, uintptr(unsafe.Pointer(valName)))

	notifyWinINet()
}

func enableEnvProxy(proxyURL string) {
	subKey, _ := syscall.UTF16PtrFromString(`Environment`)
	var hKey uintptr
	r, _, _ := procRegOpenKeyExW.Call(
		uintptr(HKEY_CURRENT_USER),
		uintptr(unsafe.Pointer(subKey)),
		0,
		uintptr(KEY_WRITE),
		uintptr(unsafe.Pointer(&hKey)),
	)
	if r != 0 {
		return
	}
	defer procRegCloseKey.Call(hKey)

	vars := []string{"ALL_PROXY", "HTTPS_PROXY", "HTTP_PROXY", "all_proxy", "https_proxy", "http_proxy"}
	val, _ := syscall.UTF16FromString(proxyURL)
	for _, v := range vars {
		vName, _ := syscall.UTF16PtrFromString(v)
		procRegSetValueExW.Call(
			hKey,
			uintptr(unsafe.Pointer(vName)),
			0,
			uintptr(REG_SZ),
			uintptr(unsafe.Pointer(&val[0])),
			uintptr(len(val)*2),
		)
	}

	noProxyVal, _ := syscall.UTF16FromString("localhost,127.0.0.1")
	npName, _ := syscall.UTF16PtrFromString("NO_PROXY")
	procRegSetValueExW.Call(
		hKey,
		uintptr(unsafe.Pointer(npName)),
		0,
		uintptr(REG_SZ),
		uintptr(unsafe.Pointer(&noProxyVal[0])),
		uintptr(len(noProxyVal)*2),
	)

	broadcastSettingChange()
}

func disableEnvProxy() {
	subKey, _ := syscall.UTF16PtrFromString(`Environment`)
	var hKey uintptr
	r, _, _ := procRegOpenKeyExW.Call(
		uintptr(HKEY_CURRENT_USER),
		uintptr(unsafe.Pointer(subKey)),
		0,
		uintptr(KEY_WRITE),
		uintptr(unsafe.Pointer(&hKey)),
	)
	if r != 0 {
		return
	}
	defer procRegCloseKey.Call(hKey)

	vars := []string{"ALL_PROXY", "HTTPS_PROXY", "HTTP_PROXY", "all_proxy", "https_proxy", "http_proxy", "NO_PROXY"}
	for _, v := range vars {
		vName, _ := syscall.UTF16PtrFromString(v)
		procRegDeleteValueW.Call(hKey, uintptr(unsafe.Pointer(vName)))
	}

	broadcastSettingChange()
}

func broadcastSettingChange() {
	user32 := syscall.NewLazyDLL("user32.dll")
	sendMessageTimeout := user32.NewProc("SendMessageTimeoutW")
	const (
		HWND_BROADCAST   = 0xffff
		WM_SETTINGCHANGE = 0x001A
		SMTO_ABORTIFHUNG = 0x0002
	)
	envStr, _ := syscall.UTF16PtrFromString("Environment")
	var result uintptr
	sendMessageTimeout.Call(
		uintptr(HWND_BROADCAST),
		uintptr(WM_SETTINGCHANGE),
		0,
		uintptr(unsafe.Pointer(envStr)),
		uintptr(SMTO_ABORTIFHUNG),
		500,
		uintptr(unsafe.Pointer(&result)),
	)
}

func saveCurrentProxySettings() {
	subKey, _ := syscall.UTF16PtrFromString(`Software\Microsoft\Windows\CurrentVersion\Internet Settings`)
	var hKey uintptr
	r, _, _ := procRegOpenKeyExW.Call(
		uintptr(HKEY_CURRENT_USER),
		uintptr(unsafe.Pointer(subKey)),
		0,
		uintptr(KEY_READ),
		uintptr(unsafe.Pointer(&hKey)),
	)
	if r != 0 {
		return
	}
	defer procRegCloseKey.Call(hKey)

	initialSettings.WasConfigured = true

	var val uint32
	var valType uint32
	valSize := uint32(4)
	valName, _ := syscall.UTF16PtrFromString("ProxyEnable")
	r, _, _ = procRegQueryValueExW.Call(
		hKey,
		uintptr(unsafe.Pointer(valName)),
		0,
		uintptr(unsafe.Pointer(&valType)),
		uintptr(unsafe.Pointer(&val)),
		uintptr(unsafe.Pointer(&valSize)),
	)
	if r == 0 {
		initialSettings.Enabled = val
	} else {
		initialSettings.Enabled = 0
	}
}

func notifyWinINet() {
	procInternetSetOptionW := syscall.NewLazyDLL("wininet.dll").NewProc("InternetSetOptionW")
	procInternetSetOptionW.Call(0, uintptr(INTERNET_OPTION_SETTINGS_CHANGED), 0, 0)
	procInternetSetOptionW.Call(0, uintptr(INTERNET_OPTION_REFRESH), 0, 0)
}

func ConfigureAdapterDNS(dnsIP string) {
	_ = exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", "Disable-NetAdapterBinding -Name * -ComponentId ms_tcpip6 -ErrorAction SilentlyContinue").Run()

	psCmd := fmt.Sprintf("Get-NetAdapter | Where-Object { $_.Status -eq 'Up' -and $_.InterfaceDescription -notlike '*Virtual*' } | ForEach-Object { Set-DnsClientServerAddress -InterfaceIndex $_.InterfaceIndex -ServerAddresses '%s' }", dnsIP)
	_ = exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", psCmd).Run()
	_ = exec.Command("netsh", "interface", "ip", "set", "dns", "name=Wi-Fi", "static", dnsIP).Run()
	_ = exec.Command("netsh", "interface", "ip", "set", "dns", "name=Ethernet", "static", dnsIP).Run()
	EnableRobloxHosts()
	FlushDNS()
}

func RestoreAdapterDNS() {
	DisableRobloxHosts()
	psCmd := "Get-NetAdapter | Where-Object { $_.Status -eq 'Up' -and $_.InterfaceDescription -notlike '*Virtual*' } | ForEach-Object { Set-DnsClientServerAddress -InterfaceIndex $_.InterfaceIndex -ResetServerAddresses }"
	_ = exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", psCmd).Run()
	_ = exec.Command("netsh", "interface", "ip", "set", "dns", "name=Wi-Fi", "dhcp").Run()
	_ = exec.Command("netsh", "interface", "ip", "set", "dns", "name=Ethernet", "dhcp").Run()
	_ = exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", "Enable-NetAdapterBinding -Name * -ComponentId ms_tcpip6 -ErrorAction SilentlyContinue").Run()
	FlushDNS()
}
