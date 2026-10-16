"""Locally decode recording originals to cached browser-compatible PCM WAV."""
import os
from pathlib import Path
import subprocess
import threading
import wave

_lock=threading.Lock()

def prepare(source):
    target=source.with_suffix('.playback.wav')
    with _lock:
        if target.is_file() and target.stat().st_mtime>=source.stat().st_mtime:
            return target
        executable=Path(__file__).resolve().parent.parent/'.tools/ffmpeg.exe'
        if not executable.is_file():raise RuntimeError('Playback converter is not installed')
        temporary=source.with_suffix('.playback.tmp.wav')
        try:
            result=subprocess.run([str(executable),'-nostdin','-v','error','-y','-i',str(source),'-map','0:a:0','-vn','-c:a','pcm_s16le','-f','wav',str(temporary)],stdin=subprocess.DEVNULL,stdout=subprocess.DEVNULL,stderr=subprocess.PIPE,timeout=600,creationflags=getattr(subprocess,'CREATE_NO_WINDOW',0))
            if result.returncode:raise RuntimeError('Recording decode failed')
            with wave.open(str(temporary),'rb') as audio:
                if audio.getnframes()==0:raise RuntimeError('Recording has no audio samples')
            os.replace(temporary,target)
            return target
        finally:temporary.unlink(missing_ok=True)
