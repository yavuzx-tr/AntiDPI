//go:build !windows

package main

import (
	"os"
	"os/exec"
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

func EnableSystemProxy(proxyAddr string) error {
	// Configure GNOME desktop proxy settings if gsettings is available
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
	// Configure GNOME desktop PAC auto-config if gsettings is available
	_ = exec.Command("gsettings", "set", "org.gnome.system.proxy", "mode", "auto").Run()
	_ = exec.Command("gsettings", "set", "org.gnome.system.proxy", "autoconfig-url", pacURL).Run()
	return nil
}

func DisablePAC() {
	_ = exec.Command("gsettings", "set", "org.gnome.system.proxy", "mode", "none").Run()
	_ = exec.Command("gsettings", "reset", "org.gnome.system.proxy", "autoconfig-url").Run()
}

func ConfigureAdapterDNS(dnsIP string) {
	// On Linux, transparent relay on 127.0.0.1:443 & :80 paired with /etc/hosts handles routing directly
}

func RestoreAdapterDNS() {
	// No-op on Linux
}
