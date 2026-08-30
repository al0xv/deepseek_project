#!/usr/bin/env bash
# Builds and runs the gateway locally.
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p bin
go build -o bin/dsgateway ./cmd/dsgateway
exec ./bin/dsgateway "$@"
