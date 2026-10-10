# AntiDPI 2.0

> **Lightweight, Driverless TCP Desync & Censorship Circumvention Engine for Windows**

[![GitHub Release](https://img.shields.io/github/v/release/yavuzx-tr/AntiDPI?style=for-the-badge&color=blue)](https://github.com/yavuzx-tr/AntiDPI/releases/latest)
[![Download Latest](https://img.shields.io/badge/Download-AntiDPI%202.0%20(ZIP)-brightgreen?style=for-the-badge&logo=windows)](https://github.com/yavuzx-tr/AntiDPI/releases/latest)

[**📦 Download Latest Release (AntiDPI-v2.0.0-windows-x64.zip)**](https://github.com/yavuzx-tr/AntiDPI/releases/latest)

AntiDPI 2.0 is a standalone, kernel-free Deep Packet Inspection (DPI) bypass utility designed specifically for Windows. It circumvents ISP-level censorship and stateful packet inspection middleboxes without requiring kernel-mode packet filtering drivers (WinDivert, Npcap), virtual network adapters, VPNs, or external proxy tunnels.

---

## ⚡ What's New in 2.0

- **Transparent SNI Relay (`127.0.0.1:443` & `127.0.0.1:80`):**
  - Intercepts and desynchronizes raw TLS ClientHello traffic from non-proxy-aware game engines and applications (e.g. Roblox `libcurl` stack).
  - Eliminates the need for manual browser restarts, proxy configuration in games, or environment variables.
- **Dynamic Wildcard DNS Synthesis (RFC 8484 DoH on `127.0.0.1:53`):**
  - Embedded local DNS resolver that intercepts dynamic game asset subdomains (`*.roblox.com`) and routes them directly through the transparent desync engine.
  - Automatically synthesizes NODATA for IPv6 AAAA requests, defeating ISP IPv6 sinkholing without connection timeouts.
- **ServerHello Handshake Verification & Multi-IP Failover:**
  - Performs non-blocking early handshake verification with instant failover across European edge endpoints (`128.116.5.3`, Cloudflare `104.18.3.63`, CloudFront `99.84.91.31`), defeating TCP RST injection.
- **Silent Engine Mode:**
  - Suppresses background telemetry noise and tracking pixel drops, keeping the console UI clean, minimalist, and focused.
- **Self-Healing & Instant Graceful Cleanup:**
  - Native Win32 `SetConsoleCtrlHandler` ensures system proxy, DNS adapters, hosts file, and firewall rules are cleanly restored within milliseconds upon exit (`Ctrl+C`, window close, or shutdown).

---

## 🚀 Quick Start

### Running AntiDPI 2.0
1. Right-click **`run.bat`** and select **Run as administrator**.
2. Once the ASCII logo and `[OK] Engine Status: Active` status box appear, the protection is live.
3. Open Roblox, Discord, or any blocked website — all connections will establish immediately.
4. You can minimize the command prompt window while playing. Closing the window automatically restores all network settings.

### Autostart on Windows Boot
- Run **`autostart.bat`**:
  - `[1]` Enable Windows Autostart (registers a task to start silently on user logon).
  - `[2]` Disable Windows Autostart (removes startup task).
  - `[3]` Reset & Fix Network (restores default DNS and proxy if ever interrupted).

---

## ⚙️ Architecture & DPI Circumvention

```
+-------------------------------------------------------------------------+
|                              WINDOWS OS                                 |
+-------------------------------------------------------------------------+
|                                                                         |
|  [Browser / WinINet] ------> Dual HTTP/SOCKS5 Proxy (127.0.0.1:8080)    |
|                                         |                               |
|  [Roblox Game Engine] -----> Transparent SNI Relay (127.0.0.1:443)      |
|  (libcurl / Winsock)                    |                               |
|                                         v                               |
|                     +---------------------------------------+           |
|                     |     COMBO DESYNC FRAGMENTATION        |           |
|                     |  - Split 1st Byte (0x16 Handshake)    |           |
|                     |  - Split SNI Midpoint (TCP_NODELAY)   |           |
|                     +---------------------------------------+           |
|                                         |                               |
+-----------------------------------------|-------------------------------+
                                          v
                              +-----------------------+
                              |   ISP MIDDLEBOX (DPI) |  <-- Unable to parse SNI
                              +-----------------------+      Passes through intact
                                          v
                              +-----------------------+
                              |   REMOTE SERVER EDGE  |  <-- Reassembles TCP stream
                              |  (Roblox / Cloudflare)|      Completes TLS Handshake
                              +-----------------------+
```

---

## 🛠️ CLI Reference

```cmd
bin\antidpi.exe [flags]
```

| Flag | Default | Description |
| :--- | :--- | :--- |
| `-port` | `8080` | Dual HTTP CONNECT & SOCKS5 proxy listening port |
| `-autoproxy` | `true` | Automatically configure Windows WinINet system proxy |
| `-dns` | `true` | Enable RFC 8484 UDP Port 53 DoH relay on `127.0.0.1:53` |

---

## 📦 Building from Source

Requires **Go 1.21+** on Windows:

```cmd
cd bin\src
go build -trimpath -ldflags "-s -w" -o ..\antidpi.exe .
```

Or run the build script:
```cmd
cd bin\src
build.bat
```

To compile with the embedded application icon, ensure `rsrc.syso` is present or generate it with:
```cmd
rsrc -manifest antidpi.manifest -ico icon.ico -o rsrc.syso
```

---

## 📄 License

This project is licensed under the [MIT License](LICENSE).
