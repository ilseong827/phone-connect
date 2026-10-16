#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$(readlink -f "$0")")"
if [[ $(uname -s) != Linux ]]; then echo 'Ubuntu/Linux required' >&2; exit 1; fi
sha256sum -c SHA256SUMS
target="$HOME/.local/lib/pocketrelay"
mkdir -p "$target" "$HOME/.local/bin"
install -m 755 pocketrelay "$target/pocketrelay.new"
mv "$target/pocketrelay.new" "$target/pocketrelay"
if [[ -f PocketRelay-debug.apk ]]; then install -m 644 PocketRelay-debug.apk "$target/"; fi
ln -sfn "$target/pocketrelay" "$HOME/.local/bin/pocketrelay"
"$target/pocketrelay" init
printf '\nInstalled: %s\n' "$HOME/.local/bin/pocketrelay"
printf 'Run: export PATH="$HOME/.local/bin:$PATH"\nThen: pocketrelay\nWeb: http://localhost:8788\n'
