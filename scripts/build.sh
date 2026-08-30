#!/usr/bin/env bash
# Cross-compiles the client and gateway for macOS and Windows.
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p bin

echo "==> building macOS binaries"
go build -o bin/ds ./cmd/ds
go build -o bin/dsgateway ./cmd/dsgateway

echo "==> building Windows binaries"
GOOS=windows GOARCH=amd64 go build -o bin/ds.exe ./cmd/ds
GOOS=windows GOARCH=amd64 go build -o bin/dsgateway.exe ./cmd/dsgateway

echo "==> done (bin/)"
