"""PC-only receiver viewer. Never attach the public tunnel to this port."""
import html
import json
import re
import sqlite3
from contextlib import closing
from datetime import datetime
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import urlsplit, parse_qs, quote
from audio_preview import prepare

ROOT = Path(__file__).resolve().parents[2]
DATA = ROOT / 'receiver-data'

def audio_mime(head):
    if head[4:8] == b'ftyp': return 'audio/mp4'
    if head[:4] == b'RIFF' and head[8:12] == b'WAVE': return 'audio/wav'
    if head.startswith(b'OggS'): return 'audio/ogg'
    if head.startswith(b'fLaC'): return 'audio/flac'
    if head.startswith(b'#!AMR'): return 'audio/amr'
    if head.startswith(b'ID3') or (len(head)>1 and head[0]==255 and head[1]&0xe0==0xe0): return 'audio/mpeg'
    return 'application/octet-stream'


def esc(value):
    return html.escape(str(value if value is not None else '확인 불가'))


def time_text(value):
    try:
        return datetime.fromtimestamp(value / 1000).strftime('%m/%d %H:%M:%S')
    except (TypeError, ValueError, OSError):
        return '확인 불가'


class Viewer(BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def do_GET(self):
        if self.headers.get('Host', '') not in ('127.0.0.1:8788', 'localhost:8788'):
            self.send_error(403)
            return
        if self.path.startswith('/audio/'):
            part=urlsplit(self.path)
            identity = part.path.removeprefix('/audio/')
            if not re.fullmatch('[0-9a-f]{64}', identity):
                self.send_error(404)
                return
            file = DATA / (identity + '.audio')
            if not file.exists():
                self.send_error(404)
                return
            if parse_qs(part.query).get('format')==['wav']:
                try:file=prepare(file)
                except Exception:
                    self.send_error(503,'Unable to prepare audio playback')
                    return
            name='녹음 파일'
            with closing(sqlite3.connect(DATA / 'events.sqlite3')) as db:
                row = db.execute('SELECT payload FROM events WHERE id=?', (identity,)).fetchone()
                if row: name=json.loads(row[0]).get('fileName') or name
            with file.open('rb') as source:mime=audio_mime(source.read(512))
            size=file.stat().st_size;start=0;end=size-1;partial=False
            requested=self.headers.get('Range')
            if requested:
                match=re.fullmatch(r'bytes=(\d*)-(\d*)',requested)
                if not match or not any(match.groups()):self.send_error(416);return
                first,last=match.groups()
                if first:start=int(first);end=min(int(last),size-1) if last else size-1
                else:start=max(0,size-int(last))
                if start>=size or start>end:
                    self.send_response(416);self.send_header('Content-Range',f'bytes */{size}');self.end_headers();return
                partial=True
            self.send_response(206 if partial else 200)
            self.send_header('Content-Type', mime)
            self.send_header('X-Content-Type-Options', 'nosniff')
            self.send_header('Accept-Ranges','bytes')
            self.send_header('Cache-Control','no-store')
            self.send_header('Content-Length', str(max(0,end-start+1)))
            if partial:self.send_header('Content-Range',f'bytes {start}-{end}/{size}')
            if parse_qs(part.query).get('download')==['1']:
                self.send_header('Content-Disposition',"attachment; filename*=UTF-8''"+quote(str(name),safe=''))
            self.end_headers()
            with file.open('rb') as source:
                source.seek(start);left=end-start+1
                try:
                    while left>0:
                        data=source.read(min(left,65536))
                        if not data:break
                        self.wfile.write(data);left-=len(data)
                except (BrokenPipeError,ConnectionResetError,ConnectionAbortedError):pass
            return
        if self.path not in ('/', '/favicon.ico'):
            self.send_error(404)
            return
        config_file = ROOT / '.tools' / 'pc-session.json'
        config = json.loads(config_file.read_text('utf-8-sig')) if config_file.exists() else {}
        url = config.get('url', 'HTTPS 주소 준비 중')
        token = config.get('token', '')
        cards = []
        total = 0
        if (DATA / 'events.sqlite3').exists():
            with closing(sqlite3.connect(DATA / 'events.sqlite3')) as db:
                total = db.execute('SELECT count(*) FROM events').fetchone()[0]
                rows = db.execute('SELECT payload FROM events ORDER BY rowid DESC LIMIT 50').fetchall()
            for row in rows:
                p = json.loads(row[0])
                if p['type'] == 'kakao':
                    cards.append('<article><span class="tag">카카오톡</span><h3>' + esc(p.get('sender')) + ' <small>· ' + esc(p.get('chatRoom')) + '</small></h3><p class="message">' + esc(p.get('message')) + '</p><footer>알림 제목: ' + esc(p.get('notificationTitle')) + ' · ' + time_text(p.get('receivedAt')) + '</footer></article>')
                else:
                    cards.append('<article><span class="tag">녹음</span><h3>' + esc(p.get('fileName')) + '</h3><p>' + esc(round(p.get('size', 0) / 1024 / 1024, 2)) + ' MB · 파일 수정 ' + time_text(p.get('fileModifiedAt')) + '</p><audio controls preload="none" src="/audio/' + p['id'] + '"></audio><p><a href="/audio/' + p['id'] + '">녹음 파일 열기</a></p></article>')
        page = '''<!doctype html><html lang="ko"><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>내 기록 · PC 수신함</title><style>
        *{box-sizing:border-box}body{margin:0;background:#f2f6f3;color:#1c3028;font:16px/1.65 system-ui,sans-serif}main{max-width:960px;margin:auto;padding:42px 24px}h1{font-size:36px;letter-spacing:-1.4px;margin:8px 0}h2{font-size:22px}small,footer{color:#63776d}article,.setup{padding:24px;margin:18px 0;border:1px solid #dae5de;border-radius:20px;background:white}.tag{font-size:12px;color:#146b56;font-weight:700;letter-spacing:1px}.message{white-space:pre-wrap}code{display:block;overflow-wrap:anywhere;background:#eef4ef;padding:12px;border-radius:8px;font-size:15px}a{color:#146b56}button,.button{display:inline-block;background:#146b56;color:white;border:0;border-radius:9px;padding:10px 18px;text-decoration:none;cursor:pointer}audio{width:100%}h3{margin:10px 0}details{margin-top:14px}.empty{padding:38px;text-align:center;color:#63776d}</style><main>
        <span class="tag">POCKET RELAY / LOCAL INBOX</span><h1>휴대폰에서, 내 PC로.</h1><p>카톡 알림과 녹음이 도착하면 이곳에서 확인할 수 있습니다.</p>
        <section class="setup"><h2>휴대폰 연결 설정</h2><p>1. 휴대폰에서 설치 파일을 내려받으세요.</p><a class="button" href="''' + esc(url) + '''/app.apk">안드로이드 APK 다운로드</a><p>2. 앱의 ‘받을 곳’에 아래 주소를 입력하세요.</p><code>''' + esc(url) + '''</code><details><summary>인증키 보기 · 휴대폰 앱에 입력</summary><code>''' + esc(token) + '''</code></details><p>3. 카톡 알림 접근을 허용하고 수집을 켠 뒤 ‘지금 확인하고 전송’을 누르세요.</p><small>이 확인 화면은 현재 PC에서만 열립니다. 수신 주소는 Cloudflare 임시 연결이며, PC와 연결 프로그램이 실행 중이어야 합니다.</small></section>
        <h2>받은 기록 · ''' + str(total) + '''건</h2><button onclick="location.reload()">새로고침</button><p><small>최신 50건 표시 · 녹음 재생이 끊기지 않도록 자동 새로고침하지 않습니다.</small></p>''' + (''.join(cards) or '<article class="empty">아직 도착한 기록이 없습니다.<br>휴대폰에서 테스트 메시지 하나를 보내 주세요.</article>') + '</main></html>'
        body = page.encode('utf-8')
        self.send_response(200)
        self.send_header('Content-Type', 'text/html; charset=utf-8')
        self.send_header('Cache-Control', 'no-store')
        self.send_header('X-Frame-Options', 'DENY')
        self.send_header('Content-Security-Policy', "default-src 'self'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; frame-ancestors 'none'")
        self.send_header('Content-Length', str(len(body)))
        self.end_headers()
        self.wfile.write(body)


if __name__ == '__main__':
    ThreadingHTTPServer(('127.0.0.1', 8788), Viewer).serve_forever()
