#!/usr/bin/env bash
# Regression check for the DS Remote Info.plist source of truth.
# Verifies required privacy usage descriptions are present and forbidden
# ones are absent. Mirrors the keys expected in the built application.
set -euo pipefail

PLIST="${1:-ios/Info.plist}"

required=(NSCameraUsageDescription NSLocalNetworkUsageDescription NSFaceIDUsageDescription)
forbidden=(NSMicrophoneUsageDescription NSPhotoLibraryUsageDescription)

for key in "${required[@]}"; do
  if ! /usr/libexec/PlistBuddy -c "Print :$key" "$PLIST" >/dev/null 2>&1; then
    echo "FAIL: required key '$key' missing in $PLIST"
    exit 1
  fi
done

for key in "${forbidden[@]}"; do
  if /usr/libexec/PlistBuddy -c "Print :$key" "$PLIST" >/dev/null 2>&1; then
    echo "FAIL: forbidden key '$key' present in $PLIST"
    exit 1
  fi
done

echo "OK: required privacy keys present, forbidden keys absent ($PLIST)"
