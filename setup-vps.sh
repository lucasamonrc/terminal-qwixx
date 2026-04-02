#!/usr/bin/env bash
set -euo pipefail

# One-time VPS setup script
# Run this on your VPS to configure the Qwixx game server
# Usage: ssh your-vps 'bash -s' < setup-vps.sh

echo "==> Setting up Qwixx game server..."

# Create system user
if ! id -u qwixx &>/dev/null; then
    echo "==> Creating qwixx user..."
    sudo useradd --system --no-create-home --shell /usr/sbin/nologin qwixx
fi

# Create directories
echo "==> Creating directories..."
sudo mkdir -p /opt/qwixx/.ssh
sudo chown -R qwixx:qwixx /opt/qwixx
sudo chmod 700 /opt/qwixx/.ssh

# Open firewall port
echo "==> Configuring firewall..."
if command -v ufw &>/dev/null; then
    sudo ufw allow 2222/tcp comment "Qwixx game server"
    echo "UFW rule added for port 2222"
elif command -v firewall-cmd &>/dev/null; then
    sudo firewall-cmd --permanent --add-port=2222/tcp
    sudo firewall-cmd --reload
    echo "firewalld rule added for port 2222"
else
    echo "WARNING: No firewall manager found. Make sure port 2222 is open."
fi

# Install systemd service
echo "==> Installing systemd service..."
cat << 'SERVICE' | sudo tee /etc/systemd/system/qwixx.service
[Unit]
Description=Qwixx Online Multiplayer Game Server
After=network.target

[Service]
Type=simple
User=qwixx
Group=qwixx
WorkingDirectory=/opt/qwixx
ExecStart=/opt/qwixx/qwixx --host 0.0.0.0 --port 2222
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/qwixx/.ssh
PrivateTmp=true

[Install]
WantedBy=multi-user.target
SERVICE

sudo systemctl daemon-reload
sudo systemctl enable qwixx

echo ""
echo "==> Setup complete!"
echo ""
echo "Next steps:"
echo "  1. Deploy the binary: scp qwixx user@host:/opt/qwixx/qwixx"
echo "  2. Start the service: sudo systemctl start qwixx"
echo "  3. Check status:      sudo systemctl status qwixx"
echo "  4. View logs:         sudo journalctl -u qwixx -f"
echo ""
echo "  5. Point qwixx.lucasacastro.cloud A record to this server's IP"
echo "  6. Players connect:   ssh -p 2222 qwixx.lucasacastro.cloud"
