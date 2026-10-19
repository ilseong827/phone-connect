#!/usr/bin/env bash
# Keep this file beside the downloaded image archive and phone-connect-data/.
set -euo pipefail
cd "$(dirname "$(readlink -f "$0")")"
engine=${CONTAINER_ENGINE:-}
if [[ -z "$engine" ]]; then
 if command -v podman >/dev/null; then engine=podman; else engine=docker; fi
fi
command -v "$engine" >/dev/null || { echo 'Install Docker or Podman first.' >&2; exit 1; }
name=${PHONE_CONNECT_NAME:-phone-connect}
image=${PHONE_CONNECT_IMAGE:-localhost/phone-connect:1.2.2}
port=${PHONE_CONNECT_PORT:-8788}
if "$engine" container inspect "$name" >/dev/null 2>&1; then
 "$engine" start "$name"
else
 if ! "$engine" image inspect "$image" >/dev/null 2>&1; then
  [[ -f phone-connect-container-amd64.tar.gz ]] || { echo 'Download phone-connect-container-amd64.tar.gz beside start.sh.' >&2; exit 1; }
  "$engine" load -i phone-connect-container-amd64.tar.gz
 fi
 mkdir -p phone-connect-data
 "$engine" run -d --name "$name" --restart=unless-stopped \
  -e "PHONE_CONNECT_WEB_PORT=$port" -p "127.0.0.1:$port:$port" -v "$(pwd)/phone-connect-data:/data" "$image"
fi
echo "Web: http://localhost:$port"
