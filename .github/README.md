# AntiDPI

A lightweight, driverless Deep Packet Inspection (DPI) circumvention tool for Windows. Inspired by GoodbyeDPI, AntiDPI bypasses censorship and stateful packet inspection middleboxes without requiring kernel-level packet filter drivers (WinDivert, Npcap), virtual network adapters, or background services.

---

## ⚡ Features

- **Driverless Architecture:** Runs entirely in user-space via standard Windows Win32 APIs (`ws2_32.dll`, `wininet.dll`, `advapi32.dll`). No unsigned kernel drivers or WinDivert required.
- **TLS ClientHello Desynchronization:** 
  - Splits TLS handshake packets across TCP segment boundaries (e.g., `Split-2`, `SNI-MidSplit`, `Micro-Chunk`) to prevent DPI middleboxes from extracting the Server Name Indication (SNI).
- **DNS-over-HTTPS (DoH) Integration:**
  - Embedded RFC 8484 DoH resolver using Cloudflare (`1.1.1.1`) and Google (`8.8.8.8`) upstream endpoints.
  - Immune to ISP DNS poisoning, spoofing, and block-page redirection sinkholes.
- **QUIC / HTTP/3 Auto-Fallback:**
  - Manages outbound UDP 443 firewall filtering during runtime, eliminating 2–3 second browser connection timeouts and forcing instantaneous HTTP/2 fallback.
- **Self-Healing Engine:**
  - Background health probes monitor connectivity and dynamically rotate fragmentation strategies if middlebox interference is detected.
- **Extended Mode (`alternate_run.bat`):**
  - Optional built-in UDP Port 53 DoH relay on `127.0.0.1:53` for system-level DNS overriding.
- **Crash-Resilient Cleanup:**
  - Utilizes native Win32 `SetConsoleCtrlHandler` routines to restore system proxy settings and firewall state within 10ms upon termination (closing window, shutdown, or `Ctrl+C`).

---

## 🚀 Quick Start

### Standard Run
Double-click **`run.bat`** (or execute from an elevated command prompt):
- Activates local DPI desynchronization proxy on `127.0.0.1:8080`.
- Automatically configures Windows system proxy for all browsers and desktop applications.

### Extended Run (UDP Port 53 DNS Relay)
Double-click **`alternate_run.bat`**:
- Starts the DPI proxy alongside an RFC 8484 UDP Port 53 DNS listener on `127.0.0.1:53`.

### Autostart on Windows Boot
- Run **`autostart.bat`**: Registers a scheduled task to launch standard AntiDPI on system login with highest privileges.
- Run **`alternate_autostart.bat`**: Registers a scheduled task to launch AntiDPI with Extended DNS mode (`-dns`) on system login.

---

## 🛠️ CLI Options

```cmd
antidpi.exe [flags]
```

| Flag | Default | Description |
| :--- | :--- | :--- |
| `-port` | `8080` | Local proxy listening port |
| `-autoproxy` | `true` | Automatically configure Windows WinINet system proxy |
| `-dns` | `false` | Enable local UDP Port 53 DoH relay on `127.0.0.1:53` |

---

## 📦 Building from Source

Requires **Go 1.21+**:

```cmd
cd src
go build -ldflags "-s -w" -o ..\antidpi.exe .
```

Or run the included batch script:
```cmd
cd src
build.bat
```

To compile with the application icon embedded, install `rsrc` and generate `rsrc.syso`:
```cmd
rsrc -manifest antidpi.manifest -ico icon.ico -o rsrc.syso
```

---

## 📄 License

This project is licensed under the [MIT License](LICENSE).
