#!/usr/bin/env bash
# Deploys the DS Remote gateway to an Oracle Cloud Always Free Ubuntu VM.
# Run FROM your trusted Mac. Uses only ssh/scp already present on macOS.
#
# Example:
#   ./deploy/oci/deploy.sh \
#     --ip 129.146.10.25 \
#     --ssh-host 129.146.10.25 \
#     --ssh-key ~/.ssh/dsh_oracle \
#     --arch amd64
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

usage() {
  cat <<'EOF'
Usage: deploy.sh [options]

Required:
  --ssh-host <host>   SSH target (IP or hostname) of the OCI VM
  --ssh-key <path>    path to the SSH private key for the VM

Options:
  --ip <ipv4>         OCI public IPv4; used to derive the sslip.io hostname
  --host <hostname>   public HTTPS hostname (overrides --ip), e.g. 129-146-10-25.sslip.io
  --arch <amd64|arm64> VM architecture (default: amd64)
  --user <user>       SSH user (default: ubuntu)
  --build             force rebuild of the Linux binary
  -h, --help          show this help
EOF
}

HOST=""
IP=""
SSH_HOST=""
SSH_KEY=""
ARCH="amd64"
USER="ubuntu"
BUILD="no"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --host) HOST="$2"; shift 2 ;;
    --ip) IP="$2"; shift 2 ;;
    --ssh-host) SSH_HOST="$2"; shift 2 ;;
    --ssh-key) SSH_KEY="$2"; shift 2 ;;
    --arch) ARCH="$2"; shift 2 ;;
    --user) USER="$2"; shift 2 ;;
    --build) BUILD="yes"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "error: unknown argument: $1" >&2; usage; exit 1 ;;
  esac
done

# Derive the temporary sslip.io hostname from the public IPv4 if not given.
if [[ -z "$HOST" && -n "$IP" ]]; then
  HOST="$(printf '%s' "$IP" | tr '.' '-').sslip.io"
fi

if [[ -z "$HOST" || -z "$SSH_HOST" || -z "$SSH_KEY" ]]; then
  echo "error: --host (or --ip), --ssh-host and --ssh-key are required" >&2
  usage
  exit 1
fi
if [[ ! -f "$SSH_KEY" ]]; then
  echo "error: SSH key not found: $SSH_KEY" >&2
  exit 1
fi
case "$ARCH" in
  amd64|arm64) ;;
  *) echo "error: unsupported --arch '$ARCH' (use amd64 or arm64)" >&2; exit 1 ;;
esac

SSH_OPTS=(-i "$SSH_KEY" -o StrictHostKeyChecking=accept-new -o ConnectTimeout=15)

# 1. Build the matching static Linux binary if absent or forced.
BIN="$REPO_ROOT/bin/dsgateway-linux-$ARCH"
if [[ "$BUILD" == "yes" || ! -x "$BIN" ]]; then
  echo "building $BIN ..."
  (cd "$REPO_ROOT" && make "build-linux-$ARCH")
fi

# 2. Stage files to upload.
STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT
cp "$BIN" "$STAGE/dsgateway"
chmod +x "$STAGE/dsgateway"
for f in install.sh run-gateway.sh dsgateway.service Caddyfile.template; do
  cp "$REPO_ROOT/deploy/oci/$f" "$STAGE/$f"
done

# 3. Upload and run the remote installer (root on the VM).
echo "uploading to $USER@$SSH_HOST ..."
ssh "${SSH_OPTS[@]}" "$USER@$SSH_HOST" "rm -rf /tmp/dsremote-deploy && mkdir -p /tmp/dsremote-deploy"
scp "${SSH_OPTS[@]}" "$STAGE/dsgateway" "$USER@$SSH_HOST:/tmp/dsremote-deploy/dsgateway"
scp "${SSH_OPTS[@]}" "$STAGE/install.sh" "$STAGE/run-gateway.sh" "$STAGE/dsgateway.service" "$STAGE/Caddyfile.template" "$USER@$SSH_HOST:/tmp/dsremote-deploy/"
ssh "${SSH_OPTS[@]}" "$USER@$SSH_HOST" "sudo bash /tmp/dsremote-deploy/install.sh /tmp/dsremote-deploy/dsgateway '$HOST'"

# 4. Verify public HTTPS health (first TLS issuance may take a moment).
echo "checking public HTTPS https://$HOST/healthz ..."
ok="no"
for i in $(seq 1 12); do
  if curl -fsS --max-time 20 "https://$HOST/healthz" | grep -q '"status":"ok"'; then
    ok="yes"
    break
  fi
  echo "  waiting for TLS/ACME ... ($i/12)"
  sleep 10
done
if [[ "$ok" != "yes" ]]; then
  echo "ERROR: public HTTPS health check did not pass for https://$HOST/healthz" >&2
  exit 1
fi

echo "deployment OK: https://$HOST"
