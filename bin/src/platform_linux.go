//go:build !windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func initConsole() {
	// Linux/Unix terminals natively support ANSI escape codes and UTF-8
}

func registerPlatformCloseHandler(cleanup func()) {
	// Handled via standard os/signal trap in main.go
}

func getHostsPath() string {
	return "/etc/hosts"
}

func FlushDNS() {
	// Flush systemd-resolved, resolvectl, or nscd caches if present
	_ = exec.Command("resolvectl", "flush-caches").Run()
	_ = exec.Command("systemd-resolve", "--flush-caches").Run()
	_ = exec.Command("nscd", "-I", "hosts").Run()
}

func enableQUICBlock() {
	// Drop outbound UDP 443 via iptables to force instant HTTP/2 fallback without browser hangs
	check := exec.Command("iptables", "-C", "OUTPUT", "-p", "udp", "--dport", "443", "-j", "DROP")
	if check.Run() != nil {
		_ = exec.Command("iptables", "-A", "OUTPUT", "-p", "udp", "--dport", "443", "-j", "DROP").Run()
	}
}

func disableQUICBlock() {
	_ = exec.Command("iptables", "-D", "OUTPUT", "-p", "udp", "--dport", "443", "-j", "DROP").Run()
}

func IsAdmin() bool {
	return os.Geteuid() == 0
}

func updateKioSlaveFile(filePath, proxyType, configScript string) {
	content, err := os.ReadFile(filePath)
	text := ""
	if err == nil {
		text = string(content)
	}

	lines := strings.Split(text, "\n")
	var newLines []string
	inProxy := false
	seenProxy := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			if inProxy {
				inProxy = false
			}
			if trimmed == "[Proxy Settings]" {
				inProxy = true
				seenProxy = true
				newLines = append(newLines, line)
				newLines = append(newLines, "ProxyType="+proxyType)
				if configScript != "" {
					newLines = append(newLines, "Proxy Config Script="+configScript)
				}
				continue
			}
		}
		if inProxy && (strings.HasPrefix(trimmed, "ProxyType=") || strings.HasPrefix(trimmed, "Proxy Config Script=")) {
			continue
		}
		newLines = append(newLines, line)
	}

	if !seenProxy {
		if len(newLines) > 0 && newLines[len(newLines)-1] != "" {
			newLines = append(newLines, "")
		}
		newLines = append(newLines, "[Proxy Settings]")
		newLines = append(newLines, "ProxyType="+proxyType)
		if configScript != "" {
			newLines = append(newLines, "Proxy Config Script="+configScript)
		}
	}

	out := strings.TrimRight(strings.Join(newLines, "\n"), "\n") + "\n"
	_ = os.WriteFile(filePath, []byte(out), 0644)
}

func configureKDEProxy(pacURL string, disable bool) {
	proxyType := "2"
	configScript := pacURL
	if disable {
		proxyType = "0"
		configScript = ""
	}

	_ = exec.Command("kwriteconfig6", "--file", "kioslaverc", "--group", "Proxy Settings", "--key", "ProxyType", proxyType).Run()
	_ = exec.Command("kwriteconfig6", "--file", "kioslaverc", "--group", "Proxy Settings", "--key", "Proxy Config Script", configScript).Run()
	_ = exec.Command("kwriteconfig5", "--file", "kioslaverc", "--group", "Proxy Settings", "--key", "ProxyType", proxyType).Run()
	_ = exec.Command("kwriteconfig5", "--file", "kioslaverc", "--group", "Proxy Settings", "--key", "Proxy Config Script", configScript).Run()

	homes, _ := os.ReadDir("/home")
	for _, entry := range homes {
		if !entry.IsDir() {
			continue
		}
		cfgDir := filepath.Join("/home", entry.Name(), ".config")
		if _, err := os.Stat(cfgDir); err != nil {
			continue
		}
		kioPath := filepath.Join(cfgDir, "kioslaverc")
		updateKioSlaveFile(kioPath, proxyType, configScript)
		_ = exec.Command("runuser", "-u", entry.Name(), "--", "kwriteconfig6", "--file", "kioslaverc", "--group", "Proxy Settings", "--key", "ProxyType", proxyType).Run()
		_ = exec.Command("runuser", "-u", entry.Name(), "--", "kwriteconfig6", "--file", "kioslaverc", "--group", "Proxy Settings", "--key", "Proxy Config Script", configScript).Run()
	}
}

func configureGNOMEProxy(pacURL string, disable bool) {
	mode := "auto"
	if disable {
		mode = "none"
	}
	_ = exec.Command("gsettings", "set", "org.gnome.system.proxy", "mode", mode).Run()
	if disable {
		_ = exec.Command("gsettings", "reset", "org.gnome.system.proxy", "autoconfig-url").Run()
	} else {
		_ = exec.Command("gsettings", "set", "org.gnome.system.proxy", "autoconfig-url", pacURL).Run()
	}
	homes, _ := os.ReadDir("/home")
	for _, entry := range homes {
		if !entry.IsDir() {
			continue
		}
		if disable {
			_ = exec.Command("runuser", "-u", entry.Name(), "--", "gsettings", "set", "org.gnome.system.proxy", "mode", "none").Run()
			_ = exec.Command("runuser", "-u", entry.Name(), "--", "gsettings", "reset", "org.gnome.system.proxy", "autoconfig-url").Run()
		} else {
			_ = exec.Command("runuser", "-u", entry.Name(), "--", "gsettings", "set", "org.gnome.system.proxy", "mode", "auto").Run()
			_ = exec.Command("runuser", "-u", entry.Name(), "--", "gsettings", "set", "org.gnome.system.proxy", "autoconfig-url", pacURL).Run()
		}
	}
}

func EnableSystemProxy(proxyAddr string) error {
	_ = exec.Command("gsettings", "set", "org.gnome.system.proxy", "mode", "manual").Run()
	_ = exec.Command("gsettings", "set", "org.gnome.system.proxy.http", "host", "127.0.0.1").Run()
	_ = exec.Command("gsettings", "set", "org.gnome.system.proxy.http", "port", "8080").Run()
	_ = exec.Command("gsettings", "set", "org.gnome.system.proxy.https", "host", "127.0.0.1").Run()
	_ = exec.Command("gsettings", "set", "org.gnome.system.proxy.https", "port", "8080").Run()
	return nil
}

func DisableSystemProxy() {
	_ = exec.Command("gsettings", "set", "org.gnome.system.proxy", "mode", "none").Run()
}

func EnablePAC(pacURL string) error {
	configureGNOMEProxy(pacURL, false)
	configureKDEProxy(pacURL, false)
	return nil
}

func DisablePAC() {
	configureGNOMEProxy("", true)
	configureKDEProxy("", true)
}

func ConfigureAdapterDNS(dnsIP string) {
	// On Linux, transparent relay on 127.0.0.1:443 & :80 paired with /etc/hosts handles routing directly
}

func RestoreAdapterDNS() {
	// No-op on Linux
}
