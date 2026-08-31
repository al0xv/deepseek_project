#!/usr/bin/env bash
# Installs/updates the DS Remote gateway on an Oracle Cloud Always Free
# Ubuntu VM. Runs ON the VM as root (idempotent):
#
#   sudo bash install.sh /tmp/dsremote-deploy/dsgateway <public-hostname>
#
# The binary is a static Go build (no Go toolchain needed on the server).
# The initial deployment is MOCK ONLY: no DeepSeek API key is required.
set -euo pipefail

if [[ $EUID -ne 0 ]]; then
  echo "install.sh must be run as root (sudo)" >&2
  exit 1
fi

BINARY_SRC="${1:-/tmp/dsremote-deploy/dsgateway}"
HOST="${2:-}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

APP_USER="dsremote"
APP_DIR="/opt/dsremote"
CONF_DIR="/etc/dsremote"
ENV_FILE="$CONF_DIR/gateway.env"

if [[ ! -x "$BINARY_SRC" ]]; then
  echo "binary not found or not executable: $BINARY_SRC" >&2
  exit 1
fi

# 1. Dedicated service user (idempotent, no interactive login).
if ! id "$APP_USER" &>/dev/null; then
  useradd --system --home-dir "$APP_DIR" --shell /usr/sbin/nologin "$APP_USER"
  echo "created system user $APP_USER"
fi

# 2. Application directories, binary and launcher.
mkdir -p "$APP_DIR" "$CONF_DIR"
install -m 0755 -o "$APP_USER" -g "$APP_USER" "$BINARY_SRC" "$APP_DIR/dsgateway"
install -m 0755 -o "$APP_USER" -g "$APP_USER" "$SCRIPT_DIR/run-gateway.sh" "$APP_DIR/run-gateway.sh"

# 3. Root-owned config (MOCK default; never a real key here).
if [[ ! -f "$ENV_FILE" ]]; then
  cat > "$ENV_FILE" <<'EOF'
# DS Remote gateway configuration (root-owned, mode 0640).
# Initial public deployment is MOCK ONLY: no DeepSeek API key required.
DS_GATEWAY_MODE=mock
DS_GATEWAY_LISTEN=127.0.0.1:8080
EOF
  chown root:root "$ENV_FILE"
  chmod 0640 "$ENV_FILE"
fi

# 4. systemd unit.
install -m 0644 "$SCRIPT_DIR/dsgateway.service" /etc/systemd/system/dsgateway.service
systemctl daemon-reload
systemctl enable dsgateway
systemctl restart dsgateway

# 5. Caddy TLS terminator (installed once, reconfigured on each deploy).
if ! command -v caddy &>/dev/null; then
  echo "installing Caddy..."
  apt-get update -qq
  apt-get install -y -qq debian-keyring debian-archive-keyring apt-transport-https curl
  curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' \
    | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
  curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' \
    | tee /etc/apt/sources.list.d/caddy-stable.list >/dev/null
  apt-get update -qq
  apt-get install -y -qq caddy
fi

if [[ -n "$HOST" ]]; then
  sed "s/{{HOSTNAME}}/$HOST/g" "$SCRIPT_DIR/Caddyfile.template" > /etc/caddy/Caddyfile
  systemctl enable caddy
  systemctl restart caddy
else
  echo "warning: no public hostname provided; Caddy configuration skipped" >&2
fi

# 6. Verify local gateway health.
sleep 2
if ! curl -fsS --max-time 5 http://127.0.0.1:8080/healthz; then
  echo "ERROR: gateway local health check failed" >&2
  journalctl -u dsgateway --no-pager -n 20 >&2 || true
  exit 1
fi

echo "install.sh OK: dsgateway active on 127.0.0.1:8080 (user $APP_USER)"
