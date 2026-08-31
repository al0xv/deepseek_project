#!/usr/bin/env bash
# Runs the dsremote gateway for the systemd service user.
# Mode is read from /etc/dsremote/gateway.env (DS_GATEWAY_MODE=mock|real).
# The initial public deployment is MOCK ONLY: no DeepSeek API key required.
# The gateway always binds 127.0.0.1:8080; public access is via Caddy :443.
set -euo pipefail

MODE="${DS_GATEWAY_MODE:-mock}"
LISTEN="${DS_GATEWAY_LISTEN:-127.0.0.1:8080}"

case "$MODE" in
  mock)
    exec /opt/dsremote/dsgateway -mock -listen "$LISTEN"
    ;;
  real)
    if [[ -z "${DEEPSEEK_API_KEY:-}" ]]; then
      echo "dsremote: DEEPSEEK_API_KEY is required when DS_GATEWAY_MODE=real" >&2
      exit 1
    fi
    exec /opt/dsremote/dsgateway -listen "$LISTEN"
    ;;
  *)
    echo "dsremote: unknown DS_GATEWAY_MODE '$MODE' (expected mock|real)" >&2
    exit 1
    ;;
esac
