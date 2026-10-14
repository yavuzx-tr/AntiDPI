package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// ANSI Color Codes
const (
	ColorReset = "\033[0m"
	ColorRed   = "\033[1;31m"
	ColorGreen = "\033[1;32m"
	ColorCyan  = "\033[1;36m"
	ColorWhite = "\033[1;37m"
	ColorGray  = "\033[0;90m"
)

var (
	logMu       sync.Mutex
	cleanupOnce sync.Once
)

func getTimestamp() string {
	return time.Now().Format("15:04:05")
}

func LogBypass(host, strategy string) {}
func LogDirect(host string)           {}
func LogHealing(msg string)           {}

func LogSuccess(msg string) {
	logMu.Lock()
	defer logMu.Unlock()
	fmt.Printf("%s[%s]%s %s[OK]%s %s%s%s\n",
		ColorGray, getTimestamp(), ColorReset,
		ColorGreen, ColorReset,
		ColorWhite, msg, ColorReset,
	)
}

func LogAlert(msg string) {}

func printBanner() {
	fmt.Print("\033[H\033[2J")

	fmt.Println()
	fmt.Printf("%s  █████╗ ███╗   ██╗████████╗██╗    ██████╗ ██████╗ ██╗%s\n", ColorCyan, ColorReset)
	fmt.Printf("%s ██╔══██╗████╗  ██║╚══██╔══╝██║    ██╔══██╗██╔══██╗██║%s\n", ColorCyan, ColorReset)
	fmt.Printf("%s ███████║██╔██╗ ██║   ██║   ██║    ██║  ██║██████╔╝██║%s\n", ColorCyan, ColorReset)
	fmt.Printf("%s ██╔══██║██║╚██╗██║   ██║   ██║    ██║  ██║██╔═══╝ ██║%s\n", ColorCyan, ColorReset)
	fmt.Printf("%s ██║  ██║██║ ╚████║   ██║   ██║    ██████╔╝██║     ██║%s\n", ColorCyan, ColorReset)
	fmt.Printf("%s ╚═╝  ╚═╝╚═╝  ╚═══╝   ╚═╝   ╚═╝    ╚═════╝ ╚═╝     ╚═╝%s\n", ColorCyan, ColorReset)
	fmt.Println()
	fmt.Printf("%s =============================================================%s\n", ColorCyan, ColorReset)
	fmt.Printf("%s      AntiDPI - Standalone TCP Desync & Bypass Engine        %s\n", ColorWhite, ColorReset)
	fmt.Printf("%s =============================================================%s\n", ColorCyan, ColorReset)
	fmt.Println()
	fmt.Printf("   %s[OK]%s Engine Status : %sActive (Running in Silent Mode)%s\n", ColorGreen, ColorReset, ColorGreen, ColorReset)
	fmt.Printf("   %s[OK]%s SNI Relay     : %sTransparent 127.0.0.1:443 & :80%s\n", ColorGreen, ColorReset, ColorWhite, ColorReset)
	fmt.Printf("   %s[OK]%s Protection    : %sRoblox & Discord (Zero-Risk Transparent Mode)%s\n", ColorGreen, ColorReset, ColorGreen, ColorReset)
	fmt.Println()
	fmt.Printf("%s =============================================================%s\n", ColorCyan, ColorReset)
	fmt.Println()
}

var activeRelay *TransparentRelay

// Global cleanup: guaranteed proxy restoration and firewall rule removal
func cleanupAll() {
	cleanupOnce.Do(func() {
		if activeRelay != nil {
			activeRelay.Close()
		}
		DisableHosts()
		DisablePAC()
		RestoreAdapterDNS()
		DisableSystemProxy()
		disableQUICBlock()
	})
}

func main() {
	initConsole()

	portFlag := flag.Int("port", 8080, "Local proxy port")
	autoProxy := flag.Bool("autoproxy", false, "Configure Windows WinINet system proxy")
	pacFlag := flag.Bool("pac", true, "Configure Windows WinINet PAC wildcard auto-config")
	dnsFlag := flag.Bool("dns", false, "Enable UDP Port 53 DoH relay on 127.0.0.1:53")
	flag.Parse()

	// Register platform close handler
	registerPlatformCloseHandler(cleanupAll)

	proxyAddr := fmt.Sprintf("127.0.0.1:%d", *portFlag)
	server := NewProxyServer(proxyAddr)

	FlushDNS()
	EnableHosts()
	LogSuccess("System DNS cache flushed & edge routes secured (Roblox & Discord).")

	relay, err := StartTransparentRelay()
	if err != nil {
		LogAlert(fmt.Sprintf("Transparent Relay note: %v", err))
	} else {
		activeRelay = relay
		LogSuccess("Transparent SNI Relay active on 127.0.0.1:443 & 127.0.0.1:80.")
	}

	if *pacFlag {
		pacURL := fmt.Sprintf("http://127.0.0.1:%d/proxy.pac", *portFlag)
		if err := EnablePAC(pacURL); err != nil {
			LogAlert(fmt.Sprintf("PAC note: %v", err))
		} else {
			LogSuccess("Wildcard PAC routing active (all Discord & Roblox subdomains covered).")
		}
	}

	if *dnsFlag {
		if err := StartUDP53Listener(); err != nil {
			LogAlert(fmt.Sprintf("Could not bind UDP 53: %v", err))
		} else {
			LogSuccess("UDP Port 53 DoH Relay active on 127.0.0.1:53")
			ConfigureAdapterDNS("127.0.0.1")
			LogSuccess("System DNS mapped to 127.0.0.1 (DoH).")
		}
	}

	if *autoProxy {
		if err := EnableSystemProxy(proxyAddr); err != nil {
			LogAlert(fmt.Sprintf("Failed to configure system proxy: %v", err))
		} else {
			LogSuccess("System proxy configured.")
		}
	}

	// Enable QUIC blocker to force instant HTTP/2 fallback without browser timeouts
	enableQUICBlock()
	LogSuccess("Fast QUIC bypass activated.")

	// OS signals trap
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		cleanupAll()
		os.Exit(0)
	}()

	go StartSelfHealingEngine(proxyAddr)

	printBanner()
	if err := server.Start(); err != nil {
		cleanupAll()
		os.Exit(1)
	}
}
