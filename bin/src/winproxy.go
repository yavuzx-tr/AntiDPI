package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
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
	procRegDeleteValueW      = modAdvapi32.NewProc("RegDeleteValueW")
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

	// Set ProxyOverride (bypass local loopback and LAN networks)
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
	enableEnvProxy("http://" + proxyAddr)
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
	disableEnvProxy()
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

const hostsPath = `C:\Windows\System32\drivers\etc\hosts`
const robloxBlockHeader = "# --- ANTIDPI ROBLOX BYPASS START ---"
const robloxBlockFooter = "# --- ANTIDPI ROBLOX BYPASS END ---"

var robloxHostsEntries = []string{
	"127.0.0.1 roblox.com",
	"127.0.0.1 www.roblox.com",
	"127.0.0.1 apis.roblox.com",
	"127.0.0.1 auth.roblox.com",
	"127.0.0.1 users.roblox.com",
	"127.0.0.1 ecsv2.roblox.com",
	"127.0.0.1 realtime-signalr.roblox.com",
	"127.0.0.1 notifications.roblox.com",
	"127.0.0.1 clientsettingscdn.roblox.com",
	"127.0.0.1 clientsettings.roblox.com",
	"127.0.0.1 gamejoin.roblox.com",
	"127.0.0.1 assetgame.roblox.com",
	"127.0.0.1 versioncompatibility.api.roblox.com",
	"127.0.0.1 setup.rbxcdn.com",
	"127.0.0.1 contentstore.roblox.com",
	"127.0.0.1 locale.roblox.com",
	"127.0.0.1 voice.roblox.com",
	"127.0.0.1 thumbnails.roblox.com",
	"127.0.0.1 avatar.roblox.com",
	"127.0.0.1 games.roblox.com",
	"127.0.0.1 presence.roblox.com",
	"127.0.0.1 friends.roblox.com",
	"127.0.0.1 assetdelivery.roblox.com",
	"127.0.0.1 contentdelivery.roblox.com",
	"127.0.0.1 privatemessages.roblox.com",
	"127.0.0.1 contacts.roblox.com",
	"127.0.0.1 metrics.roblox.com",
	"127.0.0.1 chat.roblox.com",
	"127.0.0.1 groups.roblox.com",
	"127.0.0.1 economy.roblox.com",
	"127.0.0.1 inventory.roblox.com",
	"127.0.0.1 develop.roblox.com",
	"127.0.0.1 badges.roblox.com",
}

// EnableRobloxHosts injects direct unblocked Roblox IPs into Windows hosts file to bypass ISP DNS sinkholes
func EnableRobloxHosts() {
	content, err := os.ReadFile(hostsPath)
	if err != nil {
		return
	}
	text := cleanHostsText(string(content))

	var sb strings.Builder
	sb.WriteString(strings.TrimRight(text, "\r\n"))
	sb.WriteString("\r\n\r\n")
	sb.WriteString(robloxBlockHeader)
	sb.WriteString("\r\n")
	for _, entry := range robloxHostsEntries {
		sb.WriteString(entry)
		sb.WriteString("\r\n")
	}
	sb.WriteString(robloxBlockFooter)
	sb.WriteString("\r\n")

	_ = os.WriteFile(hostsPath, []byte(sb.String()), 0644)
	FlushDNS()
}

// DisableRobloxHosts removes injected Roblox IPs from Windows hosts file
func DisableRobloxHosts() {
	content, err := os.ReadFile(hostsPath)
	if err != nil {
		return
	}
	cleaned := cleanHostsText(string(content))
	_ = os.WriteFile(hostsPath, []byte(cleaned), 0644)
	FlushDNS()
}

func cleanHostsText(text string) string {
	startIdx := strings.Index(text, robloxBlockHeader)
	if startIdx == -1 {
		return text
	}
	endIdx := strings.Index(text, robloxBlockFooter)
	if endIdx == -1 {
		return text[:startIdx]
	}
	endIdx += len(robloxBlockFooter)
	return strings.TrimRight(text[:startIdx], "\r\n") + "\r\n" + strings.TrimLeft(text[endIdx:], "\r\n")
}

// ConfigureAdapterDNS sets connected network adapters' DNS to the local DoH relay and disables IPv6 sinkholing
func ConfigureAdapterDNS(dnsIP string) {
	// Disable IPv6 on adapters to prevent ISP fake AAAA sinkhole poisoning
	_ = exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", "Disable-NetAdapterBinding -Name * -ComponentId ms_tcpip6 -ErrorAction SilentlyContinue").Run()

	psCmd := fmt.Sprintf("Get-NetAdapter | Where-Object { $_.Status -eq 'Up' -and $_.InterfaceDescription -notlike '*Virtual*' } | ForEach-Object { Set-DnsClientServerAddress -InterfaceIndex $_.InterfaceIndex -ServerAddresses '%s' }", dnsIP)
	_ = exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", psCmd).Run()
	_ = exec.Command("netsh", "interface", "ip", "set", "dns", "name=Wi-Fi", "static", dnsIP).Run()
	_ = exec.Command("netsh", "interface", "ip", "set", "dns", "name=Ethernet", "static", dnsIP).Run()
	EnableRobloxHosts()
	FlushDNS()
}

// RestoreAdapterDNS resets connected network adapters' DNS to DHCP and restores IPv6
func RestoreAdapterDNS() {
	DisableRobloxHosts()
	psCmd := "Get-NetAdapter | Where-Object { $_.Status -eq 'Up' -and $_.InterfaceDescription -notlike '*Virtual*' } | ForEach-Object { Set-DnsClientServerAddress -InterfaceIndex $_.InterfaceIndex -ResetServerAddresses }"
	_ = exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", psCmd).Run()
	_ = exec.Command("netsh", "interface", "ip", "set", "dns", "name=Wi-Fi", "dhcp").Run()
	_ = exec.Command("netsh", "interface", "ip", "set", "dns", "name=Ethernet", "dhcp").Run()
	_ = exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", "Enable-NetAdapterBinding -Name * -ComponentId ms_tcpip6 -ErrorAction SilentlyContinue").Run()
	FlushDNS()
}
