#!/usr/bin/env bash
# Builds build/PhoenixAI.exe and the installer build/PhoenixAI-Setup-<version>.exe
# (plus its .sha256 checksum, which the auto-updater checks).
#
# Needs Go and NSIS (makensis). Works on Linux, macOS and in GitHub Actions.
set -euo pipefail
cd "$(dirname "$0")"

VERSION="$(tr -d '[:space:]' < VERSION)"
echo "Building Phoenix AI $VERSION"

go run ./tools/mkres
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -tags desktop,production \
  -ldflags "-w -s -H windowsgui -X phoenixai/backend.Version=$VERSION" \
  -o build/PhoenixAI.exe .

makensis -V2 -DVERSION="$VERSION" build/installer/phoenix.nsi

SETUP="PhoenixAI-Setup-$VERSION.exe"
cd build
if command -v sha256sum >/dev/null; then
  sha256sum "$SETUP" > "$SETUP.sha256"
else
  shasum -a 256 "$SETUP" > "$SETUP.sha256"
fi
echo "Done: build/$SETUP"
