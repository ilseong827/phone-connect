"""Local-only chat UI; reuses the existing audio endpoint."""
import json
import re
import sqlite3
from contextlib import closing
from http.server import ThreadingHTTPServer
from pathlib import Path
from urllib.parse import urlsplit, parse_qs, quote
import sys
sys.path.insert(0, str(Path(__file__).resolve().parent))
from local_viewer import Viewer, ROOT, DATA
from attachments import PREVIEW_IMAGES, PREVIEW_MEDIA, clean_name


def events(after=0):
    if not (DATA / 'events.sqlite3').exists():
        return {'events': [], 'cursor': after, 'more': False}
    with closing(sqlite3.connect(DATA / 'events.sqlite3')) as db:
        rows = db.execute('SELECT rowid,payload FROM events WHERE rowid>? ORDER BY rowid LIMIT 501', (after,)).fetchall()
    batch = rows[:500]
    return {'events': [dict(json.loads(p), sequence=n) for n, p in batch],
            'cursor': batch[-1][0] if batch else after, 'more': len(rows) > 500}


class ChatViewer(Viewer):
    def reply(self, data, mime='application/json; charset=utf-8'):
        body = data if isinstance(data, bytes) else json.dumps(data, ensure_ascii=False).encode('utf-8')
        self.send_response(200)
        for key, value in {'Content-Type': mime, 'Content-Length': str(len(body)), 'Cache-Control': 'no-store',
                           'X-Content-Type-Options': 'nosniff', 'X-Frame-Options': 'DENY',
                           'Content-Security-Policy': "default-src 'self'; style-src 'unsafe-inline'; script-src 'self'; frame-ancestors 'none'; base-uri 'none'"}.items():
            self.send_header(key, value)
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        if self.headers.get('Host', '') not in ('localhost:8788', '127.0.0.1:8788'):
            self.send_error(403)
            return
        part = urlsplit(self.path)
        if part.path == '/api/events':
            try:
                after = int(parse_qs(part.query).get('after', ['0'])[0])
                if after < 0:
                    raise ValueError()
            except ValueError:
                self.send_error(400)
                return
            self.reply(events(after))
        elif part.path.startswith(('/image/','/file/')):
            identity = part.path.split('/')[-1]
            if not re.fullmatch('[0-9a-f]{64}', identity):
                self.send_error(404)
                return
            if not (DATA/'events.sqlite3').is_file():
                self.send_error(404)
                return
            file = DATA / (identity + '.blob')
            if not file.is_file(): file = DATA / (identity + '.image')
            mime = None; name = '첨부파일'
            with closing(sqlite3.connect(DATA / 'events.sqlite3')) as db:
                for (payload,) in db.execute('SELECT payload FROM events'):
                    for item in json.loads(payload).get('attachments', []):
                        if item.get('fileId',item.get('imageId')) == identity:
                            mime = item.get('mimeType')
                            name = clean_name(item.get('fileName'))
                            break
                    if mime:
                        break
            if not file.is_file() or mime is None:
                self.send_error(404)
                return
            inline = (part.path.startswith('/image/') or parse_qs(part.query).get('preview')==['1']) and mime in PREVIEW_IMAGES|PREVIEW_MEDIA
            size=file.stat().st_size;start=0;end=size-1;partial=False
            requested=self.headers.get('Range')
            if requested and size:
                match=re.fullmatch(r'bytes=(\d*)-(\d*)',requested)
                if not match or not any(match.groups()):
                    self.send_error(416);return
                first,last=match.groups()
                if first:start=int(first);end=min(int(last),size-1) if last else size-1
                else:start=max(0,size-int(last))
                if start>=size or start>end:
                    self.send_response(416);self.send_header('Content-Range',f'bytes */{size}');self.end_headers();return
                partial=True
            self.send_response(206 if partial else 200)
            self.send_header('Content-Type',mime if inline else 'application/octet-stream')
            self.send_header('Content-Disposition',("inline" if inline else "attachment")+"; filename*=UTF-8''"+quote(name,safe=''))
            self.send_header('X-Content-Type-Options','nosniff')
            self.send_header('Content-Security-Policy',"sandbox; default-src 'none'")
            self.send_header('Cache-Control','no-store');self.send_header('Accept-Ranges','bytes')
            self.send_header('Content-Length',str(max(0,end-start+1)))
            if partial:self.send_header('Content-Range',f'bytes {start}-{end}/{size}')
            self.end_headers()
            with file.open('rb') as source:
                source.seek(start);left=end-start+1
                while left>0:
                    data=source.read(min(left,65536))
                    if not data:break
                    self.wfile.write(data);left-=len(data)
        elif part.path == '/api/connection':
            file = ROOT / '.tools/pc-session.json'
            config = json.loads(file.read_text('utf-8-sig')) if file.exists() else {}
            self.reply({'url': config.get('url', ''), 'token': config.get('token', '')})
        elif part.path in ('/', '/chat.js'):
            file = 'chat.html' if part.path == '/' else 'chat.js'
            self.reply((Path(__file__).parent / file).read_bytes(), 'text/html; charset=utf-8' if file.endswith('html') else 'text/javascript; charset=utf-8')
        else:
            super().do_GET()


if __name__ == '__main__':
    ThreadingHTTPServer(('127.0.0.1', 8788), ChatViewer).serve_forever()
