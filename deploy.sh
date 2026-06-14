#!/usr/bin/env bash
set -euo pipefail

# ============================================
#  Mirabellier Go Backend — Deploy via Git + PM2
#  Run from your LOCAL machine
# ============================================

VPS_HOST="${VPS_HOST:-your-vps}"
VPS_USER="${VPS_USER:-root}"
VPS_PATH="/var/www/mirabellier-backend-go"

echo "=== 1. Building Go binary for Linux ==="
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server ./cmd/server
echo "   Done: server"

echo "=== 2. Uploading binary + config to VPS ==="
scp server                ${VPS_USER}@${VPS_HOST}:${VPS_PATH}/
scp ecosystem.config.js   ${VPS_USER}@${VPS_HOST}:${VPS_PATH}/

echo "=== 3. Restarting PM2 ==="
ssh ${VPS_USER}@${VPS_HOST} << 'ENDSSH'
set -e
cd /var/www/mirabellier-backend-go

# Create dirs if needed
mkdir -p logs images data

# Copy .env.example if .env missing
if [ ! -f .env ]; then
    cp .env.example .env
    echo "WARNING: .env created from .env.example — fill in real secrets!"
fi

# Start or restart with PM2
if pm2 list | grep -q mirabellier-go; then
    pm2 restart mirabellier-go
    echo "   Restarted existing process"
else
    pm2 start ecosystem.config.js
    pm2 save
    echo "   Started new process + saved PM2 list"
fi

pm2 status mirabellier-go
ENDSSH

echo ""
echo "=== Done ==="
echo "  Logs: ssh ${VPS_USER}@${VPS_HOST} pm2 logs mirabellier-go"
echo "  Status: ssh ${VPS_USER}@${VPS_HOST} pm2 status"
