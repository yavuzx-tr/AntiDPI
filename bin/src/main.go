package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"
	"unsafe"
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

var (
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

func LogAlert(msg string) {
	logMu.Lock()
	defer logMu.Unlock()
	fmt.Printf("%s[%s]%s %s[ALERT]%s %s%s%s\n",
		ColorGray, getTimestamp(), ColorReset,
		ColorRed, ColorReset,
		ColorRed, msg, ColorReset,
	)
}

func printBanner() {
	fmt.Println()
	fmt.Printf("%s  █████╗ ███╗   ██╗████████╗██╗    ██████╗ ██████╗ ██╗%s\n", ColorCyan, ColorReset)
	fmt.Printf("%s ██╔══██╗████╗  ██║╚══██╔══╝██║    ██╔══██╗██╔══██╗██║%s\n", ColorCyan, ColorReset)
	fmt.Printf("%s ███████║██╔██╗ ██║   ██║   ██║    ██║  ██║██████╔╝██║%s\n", ColorCyan, ColorReset)
	fmt.Printf("%s ██╔══██║██║╚██╗██║   ██║   ██║    ██║  ██║██╔═══╝ ██║%s\n", ColorCyan, ColorReset)
	fmt.Printf("%s ██║  ██║██║ ╚████║   ██║   ██║    ██████╔╝██║     ██║%s\n", ColorCyan, ColorReset)
	fmt.Printf("%s ╚═╝  ╚═╝╚═╝  ╚═══╝   ╚═╝   ╚═╝    ╚═════╝ ╚═╝     ╚═╝%s\n", ColorCyan, ColorReset)
	fmt.Println()
}

// Block QUIC (UDP 443) via Windows Firewall to force instant HTTP/2 fallback without browser timeouts
func enableQUICBlock() {
	cmd := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		"name=AntiDPI_BlockQUIC", "dir=out", "action=block", "protocol=UDP", "remoteport=443")
	_ = cmd.Run()
}

func disableQUICBlock() {
	cmd := exec.Command("netsh", "advfirewall", "firewall", "delete", "rule", "name=AntiDPI_BlockQUIC")
	_ = cmd.Run()
}

// Global cleanup: guaranteed proxy restoration and firewall rule removal
func cleanupAll() {
	cleanupOnce.Do(func() {
		DisableSystemProxy()
		disableQUICBlock()
	})
}

// Win32 Console Control Handler for closing via 'X' button or shutdown
func consoleCtrlHandler(ctrlType uint32) uintptr {
	cleanupAll()
	return 1
}

func main() {
	initConsole()

	portFlag := flag.Int("port", 8080, "Local proxy port")
	autoProxy := flag.Bool("autoproxy", true, "Configure Windows WinINet system proxy")
	dnsFlag := flag.Bool("dns", false, "Enable UDP Port 53 DoH relay on 127.0.0.1:53")
	flag.Parse()

	// Register Win32 close handler ('X' button, shutdown, logoff)
	cb := syscall.NewCallback(consoleCtrlHandler)
	procSetConsoleCtrlHandler.Call(cb, 1)

	printBanner()

	proxyAddr := fmt.Sprintf("127.0.0.1:%d", *portFlag)
	server := NewProxyServer(proxyAddr)

	FlushDNS()
	LogSuccess("Windows DNS cache flushed.")

	if *dnsFlag {
		if err := StartUDP53Listener(); err != nil {
			LogAlert(fmt.Sprintf("Could not bind UDP 53 (requires Administrator): %v", err))
		} else {
			LogSuccess("UDP Port 53 DoH Relay active on 127.0.0.1:53")
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

	LogSuccess("Protection active. Running silently in background...")
	if err := server.Start(); err != nil {
		LogAlert(fmt.Sprintf("Server error: %v", err))
		cleanupAll()
		os.Exit(1)
	}
}
