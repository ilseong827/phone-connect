#!/bin/bash
set -euo pipefail
cd "$(dirname "$(readlink -f "$0")")/.."
name=${PHONE_CONNECT_NAME:-phone-connect}
image=${PHONE_CONNECT_IMAGE:-localhost/phone-connect:1.2.1}
engine=${CONTAINER_ENGINE:-}
if [[ -z "$engine" ]]; then
 if command -v podman >/dev/null; then engine=podman; else engine=docker; fi
fi
data=${PHONE_CONNECT_DATA:-$(pwd)/phone-connect-data}
port=${PHONE_CONNECT_PORT:-8788}
action=${1:-start}
case "$action" in
 build) exec "$engine" build -t "$image" -f server/Containerfile . ;;
 run|start)
  if "$engine" container inspect "$name" >/dev/null 2>&1; then
   "$engine" start "$name"
  else
   "$engine" image inspect "$image" >/dev/null 2>&1 || "$engine" build -t "$image" -f server/Containerfile .
   mkdir -p "$data"
   data=$(cd "$data" && pwd)
   "$engine" run -d --name "$name" --restart=unless-stopped -e "PHONE_CONNECT_WEB_PORT=$port" -p "127.0.0.1:$port:$port" -v "$data:/data" "$image"
  fi
  echo "Web: http://localhost:$port"
  ;;
 stop|restart) exec "$engine" "$action" "$name" ;;
 logs) exec "$engine" logs --tail 60 "$name" ;;
 state|pair|check|dot) exec "$engine" exec "$name" phone-connect-info "$action" ;;
 *) echo 'Usage: server/run.sh [start|run|build|stop|restart|logs|state|pair|check|dot]' >&2; exit 2 ;;
esac
