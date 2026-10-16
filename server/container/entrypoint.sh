#!/bin/bash
set -euo pipefail
if [[ "${1:-all}" != all ]]; then exec "$@"; fi
umask 077
mkdir -p /data
pocketrelay init --data /data >/dev/null
pocketrelay config --data /data --model /opt/models/ggml-base.bin --whisper /usr/local/bin/whisper-cli --ffmpeg /usr/bin/ffmpeg --language ko --auto-transcribe=true >/dev/null
if [[ ! -f /data/reader.key ]]; then
 phone-connect-reader --key-file /data/reader.key --init-key
fi
if [[ "${PHONE_CONNECT_TUNNEL:-on}" == on ]]; then
 pocketrelay config --data /data --tunnel-protocol http2 >/dev/null
 pocketrelay tunnel on --data /data >/dev/null
else
 pocketrelay tunnel off --data /data >/dev/null
fi
pocketrelay serve --data /data &
server=$!
reader=''; tunnel=''
cleanup() {
 trap - TERM INT EXIT
 kill -TERM "$server" ${reader:+"$reader"} ${tunnel:+"$tunnel"} 2>/dev/null || true
 wait || true
}
trap cleanup TERM INT EXIT
for ((i=0;i<60;i++)); do
 curl -fsS http://127.0.0.1:8787/health >/dev/null 2>&1 && break
 sleep 1
done
phone-connect-reader --data /data --key-file /data/reader.key &
reader=$!
if [[ "${PHONE_CONNECT_TUNNEL:-on}" == on ]]; then
 : >/data/reader-tunnel.log
 cloudflared tunnel --no-autoupdate --protocol http2 --url http://127.0.0.1:8790 2>/data/reader-tunnel.log &
 tunnel=$!
fi
echo 'Phone Connect ready. Open the local web page and select Connection for mobile QR and Dot configuration.'
set +e
wait -n
exit 1
