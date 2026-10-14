#!/usr/bin/env bash
set -e

# Require root privileges
if [ "$EUID" -ne 0 ]; then
    echo "[!] Please run with sudo / root privileges:"
    echo "    sudo $0"
    exit 1
fi

echo "==================================================================="
echo "              AntiDPI - Linux Network Recovery Tool                "
echo "==================================================================="
echo ""

echo "[*] Terminating active AntiDPI processes..."
killall -9 antidpi 2>/dev/null || pkill -9 -x antidpi 2>/dev/null || true

echo "[*] Cleaning AntiDPI entries from /etc/hosts..."
if [ -f "/etc/hosts" ]; then
    sed -i '/# --- ANTIDPI BYPASS START ---/,/# --- ANTIDPI BYPASS END ---/d' /etc/hosts 2>/dev/null || true
    sed -i '/# --- ANTIDPI ROBLOX BYPASS START ---/,/# --- ANTIDPI ROBLOX BYPASS END ---/d' /etc/hosts 2>/dev/null || true
fi

echo "[*] Removing firewall rules (QUIC drop)..."
iptables -D OUTPUT -p udp --dport 443 -j DROP 2>/dev/null || true

echo "[*] Flushing DNS resolver caches..."
resolvectl flush-caches 2>/dev/null || systemd-resolve --flush-caches 2>/dev/null || true

echo ""
echo "==================================================================="
echo "[OK] Linux network settings restored to defaults."
echo "     All connections should function normally."
echo "==================================================================="
echo ""
