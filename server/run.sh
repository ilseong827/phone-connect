#!/bin/bash
set -euo pipefail
cd "$(dirname "$(readlink -f "$0")")/.."
name=phone-connect
image=localhost/phone-connect:1.2.0
action=${1:-start}
case "$action" in
 build) exec podman build -t "$image" -f server/Containerfile . ;;
 run|start)
  if podman container exists "$name"; then
   podman start "$name"
  else
   podman image exists "$image" || podman build -t "$image" -f server/Containerfile .
   mkdir -p phone-connect-data >/dev/null
   podman run -d --name "$name" --restart=unless-stopped -p 127.0.0.1:8788:8788 -v "$(pwd)/phone-connect-data:/data" "$image"
  fi
  echo 'Web: http://localhost:8788'
  ;;
 stop|restart) exec podman "$action" "$name" ;;
 logs) exec podman logs --tail 60 "$name" ;;
 state|pair|check|dot) exec podman exec "$name" phone-connect-info "$action" ;;
 *) echo 'Usage: server/run.sh [start|run|build|stop|restart|logs|state|pair|check|dot]' >&2; exit 2 ;;
esac
