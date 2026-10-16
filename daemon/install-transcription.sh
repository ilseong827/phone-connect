#!/usr/bin/env bash
set -euo pipefail
model="${1:-base}"
case "$model" in tiny|base|small|medium) ;; *) echo 'Model must be tiny, base, small or medium (multilingual)' >&2; exit 1;; esac
for command in git cmake c++ curl ffmpeg; do
  command -v "$command" >/dev/null || { echo 'Install dependencies: sudo apt-get install -y ffmpeg cmake g++ git curl' >&2; exit 1; }
done
root="$HOME/.local/share/pocketrelay-speech"
mkdir -p "$root"
if [[ ! -d "$root/whisper.cpp/.git" ]]; then git clone --depth 1 --branch v1.8.3 https://github.com/ggml-org/whisper.cpp.git "$root/whisper.cpp"; fi
cd "$root/whisper.cpp"
[[ $(git rev-parse HEAD) == 2eeeba56e9edd762b4b38467bab96c2517163158 ]] || { echo 'Unexpected whisper.cpp revision' >&2; exit 1; }
cmake -S . -B build -DCMAKE_BUILD_TYPE=Release -DGGML_NATIVE=OFF -DBUILD_SHARED_LIBS=OFF -DWHISPER_BUILD_TESTS=OFF -DWHISPER_BUILD_EXAMPLES=ON
cmake --build build --config Release --target whisper-cli -j 2
bash models/download-ggml-model.sh "$model"
relay="${POCKETRELAY_BIN:-$HOME/.local/bin/pocketrelay}"
"$relay" config --whisper "$root/whisper.cpp/build/bin/whisper-cli" --model "$root/whisper.cpp/models/ggml-$model.bin" --ffmpeg "$(command -v ffmpeg)" --language ko --auto-transcribe=true
echo 'Installed local transcription. Run: pocketrelay restart; pocketrelay transcribe --all'
