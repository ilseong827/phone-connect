#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$(readlink -f "$0")")"
go test ./...
go vet ./...
for arch in amd64 arm64; do
  target="../../dist/pocketrelay-linux-$arch"
  mkdir -p "$target"
  CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath -ldflags='-s -w' -o "$target/pocketrelay" .
  CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath -ldflags='-s -w' -o "$target/phone-connect-reader" ./cmd/phone-connect-reader
  cp install.sh install-transcription.sh install-tunnel.sh README-UBUNTU.md DOT-SETUP.txt THIRD-PARTY-NOTICES.txt "$target/"
  cp ../MCP.md "$target/"
  cp ../../dist/PocketRelay-debug.apk "$target/"
  chmod 755 "$target"/*.sh "$target/pocketrelay"
  (cd "$target"; sha256sum pocketrelay phone-connect-reader PocketRelay-debug.apk *.sh README-UBUNTU.md DOT-SETUP.txt THIRD-PARTY-NOTICES.txt MCP.md > SHA256SUMS)
  tar -C ../../dist -czf "../../dist/pocketrelay-linux-$arch.tar.gz" "pocketrelay-linux-$arch"
done
