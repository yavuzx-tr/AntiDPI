# AntiDPI

> **Lightweight, Driverless TCP Desync & Censorship Circumvention Engine for Windows & Linux**

[![GitHub Release](https://img.shields.io/github/v/release/yavuzx-tr/AntiDPI?style=flat-square&color=blue)](https://github.com/yavuzx-tr/AntiDPI/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg?style=flat-square)](LICENSE)
[![Platform: Windows](https://img.shields.io/badge/Platform-Windows%2010%2F11-0078d7?style=flat-square&logo=windows)](https://github.com/yavuzx-tr/AntiDPI/releases)
[![Platform: Linux](https://img.shields.io/badge/Platform-Linux%20x86__64-FCC624?style=flat-square&logo=linux&logoColor=black)](https://github.com/yavuzx-tr/AntiDPI/releases)

AntiDPI is a lightweight, standalone, driverless Deep Packet Inspection (DPI) circumvention utility designed for Windows and Linux. It bypasses ISP-level censorship and stateful packet inspection middleboxes without requiring kernel-level packet filter drivers (WinDivert, Npcap), virtual network adapters, VPNs, or external proxy tunnels.

Precompiled standalone binaries and distribution archives are available under [**Releases**](https://github.com/yavuzx-tr/AntiDPI/releases).

---

## ⚡ Features

- **Driverless Architecture:** Runs entirely in user-space without kernel drivers, WinDivert, or TUN/TAP adapters.
- **TLS ClientHello Desynchronization:**
  - Slices TLS handshake records across TCP segment boundaries using combo fragmentation (`Split-1`, `SNI-MidSplit`, `Micro-Chunk`) to prevent DPI middleboxes from detecting the Server Name Indication (SNI).
- **Transparent SNI Relay (`127.0.0.1:443` & `127.0.0.1:80`):**
  - Seamlessly intercepts and desynchronizes raw TLS traffic from non-proxy-aware game engines and desktop applications (e.g. Roblox, Discord) without manual proxy configuration.
- **DNS-over-HTTPS (DoH) Resolver (RFC 8484):**
  - Embedded local DNS resolver listening on `127.0.0.1:53` with upstream Cloudflare (`1.1.1.1`) and Google (`8.8.8.8`) DoH endpoints.
  - Immune to ISP DNS poisoning, spoofing, and block-page redirection sinkholes.
- **QUIC / UDP 443 Blocking:**
  - Automatically manages outbound UDP 443 firewall filtering (`netsh` on Windows, `iptables` on Linux) during runtime to force instantaneous HTTP/2 fallback without browser timeouts.
- **Self-Healing & Failover Engine:**
  - Performs early ServerHello verification and dynamically failovers across edge endpoints if middlebox interference is detected.
- **Crash-Resilient Cleanup:**
  - Guaranteed restoration of hosts entries, proxy settings, and firewall rules within milliseconds upon termination (`Ctrl+C`, window close, or shutdown).

---

## 🚀 Quick Start

### Windows
1. Download `AntiDPI-v1.0.4-windows-x64.zip` from [**Releases**](https://github.com/yavuzx-tr/AntiDPI/releases).
2. Extract the archive.
3. **Choose your launch mode:**
   - **Interactive Console Mode:** Right-click **`run.bat`** and select **Run as administrator**.
   - **Silent Background Autostart (No window, starts on boot):** Right-click **`autostart.bat`**, select **Run as administrator**, and choose `[1] Enable Windows Silent Autostart`.
   - **Silent Background Run Now:** Double-click **`run_silent.vbs`** or run `autostart.bat` -> option `[2]`.
   - **PM2 Users:** Run `pm2 start ecosystem.config.js`.
4. To stop or reset network settings: Run **`reset.bat`** as administrator.

### Linux
1. Download `AntiDPI-v1.0.4-linux-x64.tar.gz` from [**Releases**](https://github.com/yavuzx-tr/AntiDPI/releases).
2. Extract the archive:
   ```bash
   tar -xvf AntiDPI-v1.0.4-linux-x64.tar.gz
   cd AntiDPI
   ```
3. **Choose your launch mode:**
   - **Background Daemon & Boot Autostart with PM2 (Recommended - No open terminal):**
     ```bash
     chmod +x pm2.sh run.sh reset.sh
     ./pm2.sh autostart
     ```
     - View status: `./pm2.sh status` or `sudo pm2 status`
     - View logs: `./pm2.sh logs` or `sudo pm2 logs antidpi`
     - Stop: `./pm2.sh stop`
   - **Interactive Terminal Mode:**
     ```bash
     sudo ./run.sh
     ```
4. To stop and restore all network settings: Press `Ctrl+C` or execute `sudo ./reset.sh`.

---

## ⚙️ Architecture & DPI Circumvention

```
+-------------------------------------------------------------------------+
|                              WINDOWS OS                                 |
+-------------------------------------------------------------------------+
|                                                                         |
|  [Browser / WinINet] ------> Dual HTTP/SOCKS5 Proxy (127.0.0.1:8080)    |
|                                         |                               |
|  [Games / libcurl] --------> Transparent SNI Relay (127.0.0.1:443)      |
|  (Roblox, Discord)                      |                               |
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

### Windows
```cmd
cd bin\src
go build -trimpath -ldflags "-s -w" -o ..\antidpi.exe .
```

### Linux
```bash
cd bin/src
go build -trimpath -ldflags "-s -w" -o ../antidpi .
```

To compile with the application icon embedded:
```cmd
rsrc -manifest antidpi.manifest -ico icon.ico -o rsrc.syso
```

---

## 📄 License

This project is licensed under the [MIT License](LICENSE).
