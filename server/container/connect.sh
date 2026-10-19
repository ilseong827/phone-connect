#!/bin/bash
set -euo pipefail
case "${1:-pair}" in
 pair) exec pocketrelay --pair --data /data ;;
 state) exec pocketrelay --state --data /data ;;
 check) exec pocketrelay --check-link --data /data ;;
 dot)
  url=$(curl -fsS "http://127.0.0.1:${PHONE_CONNECT_WEB_PORT:-8788}/api/mcp-config" | sed -n 's/.*"publicUrl":"\([^"]*\)".*/\1/p' || true)
  if [[ -z "$url" ]]; then
   url=$(grep -Eo 'https://[a-z0-9-]+\.trycloudflare\.com' /data/reader-tunnel.log 2>/dev/null | tail -1 || true)
  fi
  if [[ -z "$url" ]]; then echo 'Reader tunnel URL not ready' >&2; exit 1; fi
  key=$(cat /data/reader.key)
  printf '{"url":"%s","readKey":"%s"}\n' "$url" "$key"
  ;;
 *) echo 'Usage: phone-connect-info [pair|state|check|dot]' >&2; exit 2 ;;
esac
