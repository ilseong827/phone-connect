#!/usr/bin/env bash
set -euo pipefail
case "$(uname -m)" in x86_64) arch=amd64;; aarch64|arm64) arch=arm64;; *) echo 'Unsupported architecture' >&2; exit 1;; esac
mkdir -p "$HOME/.local/bin"
temp=$(mktemp)
trap 'rm -f "$temp"' EXIT
curl --fail --location --proto '=https' --tlsv1.2 "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-$arch" --output "$temp"
install -m 755 "$temp" "$HOME/.local/bin/cloudflared"
"${POCKETRELAY_BIN:-$HOME/.local/bin/pocketrelay}" config --cloudflared "$HOME/.local/bin/cloudflared"
echo 'Installed official cloudflared. Add ~/.local/bin to PATH, then run: pocketrelay tunnel on; pocketrelay restart; pocketrelay pair'
echo 'Quick tunnel URL changes after restart. Use a stable HTTPS reverse proxy for permanent phone pairing.'
