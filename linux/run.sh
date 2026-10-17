#!/usr/bin/env bash
set -e

# Require root privileges
if [ "$EUID" -ne 0 ]; then
    echo "[!] Please run AntiDPI with sudo / root privileges:"
    echo "    sudo $0 $@"
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_PATH="$SCRIPT_DIR/bin/antidpi"

if [ ! -f "$BIN_PATH" ]; then
    if [ -f "$SCRIPT_DIR/antidpi" ]; then
        BIN_PATH="$SCRIPT_DIR/antidpi"
    elif [ -f "$SCRIPT_DIR/antidpi_linux" ]; then
        BIN_PATH="$SCRIPT_DIR/antidpi_linux"
    elif [ -f "$SCRIPT_DIR/bin/antidpi_linux" ]; then
        BIN_PATH="$SCRIPT_DIR/bin/antidpi_linux"
    fi
fi

chmod +x "$BIN_PATH" 2>/dev/null || true

cleanup() {
    echo ""
    echo "[*] Restoring network settings..."
    if [ -f "/etc/hosts" ]; then
        sed -i '/# --- ANTIDPI BYPASS START ---/,/# --- ANTIDPI BYPASS END ---/d' /etc/hosts 2>/dev/null || true
        sed -i '/# --- ANTIDPI ROBLOX BYPASS START ---/,/# --- ANTIDPI ROBLOX BYPASS END ---/d' /etc/hosts 2>/dev/null || true
    fi
    iptables -D OUTPUT -p udp --dport 443 -j DROP 2>/dev/null || true
    resolvectl flush-caches 2>/dev/null || systemd-resolve --flush-caches 2>/dev/null || true
}

trap cleanup EXIT INT TERM

echo ""
echo "  [/] Starting AntiDPI Engine (Linux Transparent Mode)..."
echo ""

"$BIN_PATH" -pac=false -autoproxy=false -dns=false "$@"
