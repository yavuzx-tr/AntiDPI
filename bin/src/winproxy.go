package main

import (
	"fmt"
	"os/exec"
	"syscall"
	"unsafe"
)

var (
	modWininet               = syscall.NewLazyDLL("wininet.dll")
	procInternetSetOptionW   = modWininet.NewProc("InternetSetOptionW")
	modAdvapi32              = syscall.NewLazyDLL("advapi32.dll")
	procRegOpenKeyExW        = modAdvapi32.NewProc("RegOpenKeyExW")
	procRegQueryValueExW     = modAdvapi32.NewProc("RegQueryValueExW")
	procRegSetValueExW       = modAdvapi32.NewProc("RegSetValueExW")
	procRegCloseKey          = modAdvapi32.NewProc("RegCloseKey")
	procCheckTokenMembership = modAdvapi32.NewProc("CheckTokenMembership")
	modDnsapi                = syscall.NewLazyDLL("dnsapi.dll")
	procDnsFlushCache        = modDnsapi.NewProc("DnsFlushResolverCache")
)

// FlushDNS clears Windows local DNS resolver cache to wipe ISP poisoned records
func FlushDNS() {
	procDnsFlushCache.Call()
}

const (
	HKEY_CURRENT_USER               = 0x80000001
	KEY_READ                        = 0x20019
	KEY_WRITE                       = 0x20006
	REG_DWORD                       = 4
	REG_SZ                          = 1
	INTERNET_OPTION_SETTINGS_CHANGED = 39
	INTERNET_OPTION_REFRESH         = 37
)

type SavedProxySettings struct {
	Enabled       uint32
	Server        string
	Override      string
	WasConfigured bool
}

var initialSettings SavedProxySettings

// IsAdmin checks if process has elevated administrator privileges
func IsAdmin() bool {
	var adminSID *byte
	// SECURITY_NT_AUTHORITY
	ntAuth := [6]byte{0, 0, 0, 0, 0, 5}
	// Allocate SECURITY_BUILTIN_DOMAIN_RID + DOMAIN_ALIAS_RID_ADMINS (544)
	r, _, err := syscall.NewLazyDLL("advapi32.dll").NewProc("AllocateAndInitializeSid").Call(
		uintptr(unsafe.Pointer(&ntAuth[0])),
		2,
		uintptr(32),  // SECURITY_BUILTIN_DOMAIN_RID
		uintptr(544), // DOMAIN_ALIAS_RID_ADMINS
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

// EnableSystemProxy configures WinINet registry and broadcasts notification
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

	// Set ProxyEnable = 1
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

	// Set ProxyServer
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

	// Set ProxyOverride
	overrideVal, _ := syscall.UTF16FromString("<-loopback>;<local>;localhost;127.*;10.*;192.168.*")
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
	return nil
}

// DisableSystemProxy restores previous proxy settings and guarantees ProxyEnable is 0
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

	// ALWAYS force ProxyEnable = 0 on exit
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

	// Clear ProxyServer if was localhost proxy
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

	// Read ProxyEnable
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
	procInternetSetOptionW.Call(0, uintptr(INTERNET_OPTION_SETTINGS_CHANGED), 0, 0)
	procInternetSetOptionW.Call(0, uintptr(INTERNET_OPTION_REFRESH), 0, 0)
}

// ConfigureAdapterDNS sets connected network adapters' DNS to the local DoH relay
func ConfigureAdapterDNS(dnsIP string) {
	psCmd := fmt.Sprintf("Get-NetAdapter | Where-Object { $_.Status -eq 'Up' -and $_.InterfaceDescription -notlike '*Virtual*' } | ForEach-Object { Set-DnsClientServerAddress -InterfaceIndex $_.InterfaceIndex -ServerAddresses '%s' }", dnsIP)
	_ = exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", psCmd).Run()
	_ = exec.Command("netsh", "interface", "ip", "set", "dns", "name=Wi-Fi", "static", dnsIP).Run()
	_ = exec.Command("netsh", "interface", "ip", "set", "dns", "name=Ethernet", "static", dnsIP).Run()
	FlushDNS()
}

// RestoreAdapterDNS resets connected network adapters' DNS to DHCP
func RestoreAdapterDNS() {
	psCmd := "Get-NetAdapter | Where-Object { $_.Status -eq 'Up' -and $_.InterfaceDescription -notlike '*Virtual*' } | ForEach-Object { Set-DnsClientServerAddress -InterfaceIndex $_.InterfaceIndex -ResetServerAddresses }"
	_ = exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", psCmd).Run()
	_ = exec.Command("netsh", "interface", "ip", "set", "dns", "name=Wi-Fi", "dhcp").Run()
	_ = exec.Command("netsh", "interface", "ip", "set", "dns", "name=Ethernet", "dhcp").Run()
	FlushDNS()
}
