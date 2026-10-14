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

// EnablePAC configures Windows Internet Settings to use the local AutoConfigURL PAC file
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

	// Keep ProxyEnable = 0 so general internet traffic is never hijacked
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

// DisablePAC removes AutoConfigURL from Windows Internet Settings
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
const antidpiBlockHeader = "# --- ANTIDPI BYPASS START ---"
const antidpiBlockFooter = "# --- ANTIDPI BYPASS END ---"
const legacyRobloxBlockHeader = "# --- ANTIDPI ROBLOX BYPASS START ---"
const legacyRobloxBlockFooter = "# --- ANTIDPI ROBLOX BYPASS END ---"

var hostsEntries = []string{
	// Roblox Web & Client
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
	"127.0.0.1 accountsettings.roblox.com",
	"127.0.0.1 accountinformation.roblox.com",
	"127.0.0.1 ephemeralcounters.api.roblox.com",
	"127.0.0.1 gameinternationalization.roblox.com",

	// Roblox Catalog, Item Configuration & Matchmaking
	"127.0.0.1 catalog.roblox.com",
	"127.0.0.1 itemconfiguration.roblox.com",
	"127.0.0.1 itemconfiguration.api.roblox.com",
	"127.0.0.1 matchmaking.roblox.com",
	"127.0.0.1 client-telemetry.roblox.com",
	"127.0.0.1 trades.roblox.com",
	"127.0.0.1 premiumfeatures.roblox.com",
	"127.0.0.1 translations.roblox.com",
	"127.0.0.1 followings.roblox.com",
	"127.0.0.1 engagement.roblox.com",
	"127.0.0.1 private-messages.roblox.com",
	"127.0.0.1 share.roblox.com",
	"127.0.0.1 devforum.roblox.com",
	"127.0.0.1 create.roblox.com",
	"127.0.0.1 talent.roblox.com",
	"127.0.0.1 education.roblox.com",
	"127.0.0.1 help.roblox.com",
	"127.0.0.1 en.help.roblox.com",
	"127.0.0.1 corp.roblox.com",
	"127.0.0.1 rbx.com",
	"127.0.0.1 www.rbx.com",
	"127.0.0.1 robloxlabs.com",

	// Roblox 3D Avatar Rendering & Asset CDNs
	"127.0.0.1 tr.rbxcdn.com",
	"127.0.0.1 images.rbxcdn.com",
	"127.0.0.1 static.rbxcdn.com",
	"127.0.0.1 js.rbxcdn.com",
	"127.0.0.1 css.rbxcdn.com",
	"127.0.0.1 c0.rbxcdn.com",
	"127.0.0.1 c1.rbxcdn.com",
	"127.0.0.1 c2.rbxcdn.com",
	"127.0.0.1 c3.rbxcdn.com",
	"127.0.0.1 c4.rbxcdn.com",
	"127.0.0.1 c5.rbxcdn.com",
	"127.0.0.1 c6.rbxcdn.com",
	"127.0.0.1 c7.rbxcdn.com",
	"127.0.0.1 t0.rbxcdn.com",
	"127.0.0.1 t1.rbxcdn.com",
	"127.0.0.1 t2.rbxcdn.com",
	"127.0.0.1 t3.rbxcdn.com",
	"127.0.0.1 t4.rbxcdn.com",
	"127.0.0.1 t5.rbxcdn.com",
	"127.0.0.1 t6.rbxcdn.com",
	"127.0.0.1 t7.rbxcdn.com",

	// Discord Web, Client & Gateway
	"127.0.0.1 discord.com",
	"127.0.0.1 www.discord.com",
	"127.0.0.1 support.discord.com",
	"127.0.0.1 support-dev.discord.com",
	"127.0.0.1 blog.discord.com",
	"127.0.0.1 status.discord.com",
	"127.0.0.1 canary.discord.com",
	"127.0.0.1 ptb.discord.com",
	"127.0.0.1 streamkit.discord.com",
	"127.0.0.1 safety.discord.com",
	"127.0.0.1 developer.discord.com",
	"127.0.0.1 admin.discord.com",
	"127.0.0.1 updates.discord.com",
	"127.0.0.1 dl.discord.com",
	"127.0.0.1 cdn.discord.com",
	"127.0.0.1 media.discord.com",
	"127.0.0.1 router.discord.com",
	"127.0.0.1 discordapp.com",
	"127.0.0.1 www.discordapp.com",
	"127.0.0.1 support.discordapp.com",
	"127.0.0.1 status.discordapp.com",
	"127.0.0.1 canary.discordapp.com",
	"127.0.0.1 ptb.discordapp.com",
	"127.0.0.1 gateway.discord.gg",
	"127.0.0.1 cdn.discordapp.com",
	"127.0.0.1 media.discordapp.net",
	"127.0.0.1 images-ext-1.discordapp.net",
	"127.0.0.1 images-ext-2.discordapp.net",
	"127.0.0.1 router.discordapp.net",
	"127.0.0.1 updates.discordapp.com",
	"127.0.0.1 dl.discordapp.net",
	"127.0.0.1 discord.gg",
	"127.0.0.1 discord.media",
	"127.0.0.1 latency.discord.media",
	"127.0.0.1 dis.gd",
	"127.0.0.1 discordstatus.com",
	"127.0.0.1 discordmerch.com",
	"127.0.0.1 discord.co",
}

// EnableHosts injects direct unblocked Roblox and Discord IPs into Windows hosts file to bypass ISP DNS sinkholes
func EnableHosts() {
	content, err := os.ReadFile(hostsPath)
	if err != nil {
		return
	}
	text := cleanHostsText(string(content))

	var sb strings.Builder
	sb.WriteString(strings.TrimRight(text, "\r\n"))
	sb.WriteString("\r\n\r\n")
	sb.WriteString(antidpiBlockHeader)
	sb.WriteString("\r\n")
	for _, entry := range hostsEntries {
		sb.WriteString(entry)
		sb.WriteString("\r\n")
	}
	sb.WriteString(antidpiBlockFooter)
	sb.WriteString("\r\n")

	_ = os.WriteFile(hostsPath, []byte(sb.String()), 0644)
	FlushDNS()
}

// DisableHosts removes injected Roblox and Discord IPs from Windows hosts file
func DisableHosts() {
	content, err := os.ReadFile(hostsPath)
	if err != nil {
		return
	}
	cleaned := cleanHostsText(string(content))
	_ = os.WriteFile(hostsPath, []byte(cleaned), 0644)
	FlushDNS()
}

func EnableRobloxHosts()  { EnableHosts() }
func DisableRobloxHosts() { DisableHosts() }

func cleanBlock(text, header, footer string) string {
	startIdx := strings.Index(text, header)
	if startIdx == -1 {
		return text
	}
	endIdx := strings.Index(text, footer)
	if endIdx == -1 {
		return text[:startIdx]
	}
	endIdx += len(footer)
	return strings.TrimRight(text[:startIdx], "\r\n") + "\r\n" + strings.TrimLeft(text[endIdx:], "\r\n")
}

func cleanHostsText(text string) string {
	text = cleanBlock(text, antidpiBlockHeader, antidpiBlockFooter)
	text = cleanBlock(text, legacyRobloxBlockHeader, legacyRobloxBlockFooter)
	return text
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
