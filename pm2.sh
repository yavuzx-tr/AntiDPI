#!/usr/bin/env bash
# AntiDPI PM2 Process & Autostart Manager for Linux

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Ensure PM2 is in PATH
export PATH="/home/tux/.local/node/bin:/home/tux/.local/bin:$HOME/.local/bin:/usr/local/bin:$PATH"

if ! command -v pm2 >/dev/null 2>&1; then
    echo "[!] PM2 is not installed or not in PATH."
    echo "    Please install PM2 first (e.g. npm install -g pm2)."
    exit 1
fi

ACTION="$1"

run_start() {
    echo "[*] Starting AntiDPI in background with PM2 (daemon mode)..."
    sudo env "PATH=$PATH" pm2 delete antidpi 2>/dev/null || true
    sudo env "PATH=$PATH" pm2 start "$SCRIPT_DIR/ecosystem.config.js"
    echo ""
    echo "[OK] AntiDPI is running in the background. No terminal window will stay open."
    echo "     View status: ./pm2.sh status"
    echo "     View logs  : ./pm2.sh logs"
}

run_autostart() {
    echo "[*] Saving active PM2 processes and configuring system boot autostart..."
    sudo env "PATH=$PATH" pm2 save
    # Configure startup for systemd
    STARTUP_CMD=$(sudo env "PATH=$PATH" pm2 startup systemd -u root --hp /root | grep "sudo env PATH" || true)
    if [ -n "$STARTUP_CMD" ]; then
        eval "$STARTUP_CMD" >/dev/null 2>&1 || true
    fi
    sudo env "PATH=$PATH" pm2 save
    echo ""
    echo "[OK] System boot autostart enabled!"
    echo "     AntiDPI will automatically start silently in the background on every reboot."
}

run_stop() {
    echo "[*] Stopping AntiDPI daemon..."
    sudo env "PATH=$PATH" pm2 stop antidpi 2>/dev/null || true
    # Run cleanup to restore /etc/hosts and firewall
    if [ -f "$SCRIPT_DIR/reset.sh" ]; then
        sudo "$SCRIPT_DIR/reset.sh" >/dev/null 2>&1 || true
    fi
    echo "[OK] AntiDPI stopped and network settings restored."
}

run_restart() {
    echo "[*] Restarting AntiDPI daemon..."
    sudo env "PATH=$PATH" pm2 restart antidpi
    echo "[OK] AntiDPI restarted."
}

run_status() {
    sudo env "PATH=$PATH" pm2 status antidpi
}

run_logs() {
    sudo env "PATH=$PATH" pm2 logs antidpi
}

run_remove() {
    echo "[*] Removing AntiDPI from PM2 and startup..."
    sudo env "PATH=$PATH" pm2 delete antidpi 2>/dev/null || true
    sudo env "PATH=$PATH" pm2 save 2>/dev/null || true
    if [ -f "$SCRIPT_DIR/reset.sh" ]; then
        sudo "$SCRIPT_DIR/reset.sh" >/dev/null 2>&1 || true
    fi
    echo "[OK] AntiDPI removed from PM2 and network settings restored."
}

case "$ACTION" in
    start)
        run_start
        ;;
    autostart|startup|enable)
        run_start
        run_autostart
        ;;
    stop)
        run_stop
        ;;
    restart)
        run_restart
        ;;
    status)
        run_status
        ;;
    logs)
        run_logs
        ;;
    remove|uninstall|delete)
        run_remove
        ;;
    *)
        echo "==================================================================="
        echo "              AntiDPI - PM2 Daemon & Autostart Manager             "
        echo "==================================================================="
        echo ""
        echo "  [1] Start in Background (PM2 Daemon - No terminal window)"
        echo "  [2] Enable System Boot Autostart (Start automatically on boot)"
        echo "  [3] Stop AntiDPI & Restore Network"
        echo "  [4] Restart AntiDPI"
        echo "  [5] Check PM2 Status"
        echo "  [6] View Realtime Logs"
        echo "  [7] Remove from PM2 & Startup"
        echo "  [8] Exit"
        echo ""
        echo "==================================================================="
        read -r -p "Select an option [1-8]: " choice
        case "$choice" in
            1) run_start ;;
            2) run_start; run_autostart ;;
            3) run_stop ;;
            4) run_restart ;;
            5) run_status ;;
            6) run_logs ;;
            7) run_remove ;;
            *) exit 0 ;;
        esac
        ;;
esac
