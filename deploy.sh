#!/usr/bin/env bash
set -euo pipefail

# Manual deploy script - backup for CI/CD
# Usage: ./deploy.sh [user@host]

VPS="${1:-${VPS_HOST:-}}"

if [ -z "$VPS" ]; then
    echo "Usage: ./deploy.sh user@host"
    echo "Or set VPS_HOST environment variable"
    exit 1
fi

echo "==> Building for Linux..."
GOOS=linux GOARCH=amd64 go build -o qwixx .

echo "==> Copying binary to $VPS..."
scp ./qwixx "${VPS}:/tmp/qwixx.new"

echo "==> Restarting service..."
ssh "$VPS" '
    sudo mv /tmp/qwixx.new /opt/qwixx/qwixx
    sudo chown qwixx:qwixx /opt/qwixx/qwixx
    sudo chmod +x /opt/qwixx/qwixx
    sudo systemctl restart qwixx
    sleep 2
    if sudo systemctl is-active --quiet qwixx; then
        echo "Deploy successful!"
    else
        echo "Deploy failed!"
        sudo journalctl -u qwixx --no-pager -n 20
        exit 1
    fi
'

# Clean up local binary
rm -f qwixx

echo "==> Done!"
