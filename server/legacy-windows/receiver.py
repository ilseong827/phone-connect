"""Local protocol reference. Put behind an HTTPS reverse proxy for phone access."""
import hashlib
import base64
import hmac
import json
import os
import re
import sqlite3
import tempfile
from contextlib import closing
from email.parser import BytesParser
from email.policy import default
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path
import sys
sys.path.insert(0, str(Path(__file__).resolve().parent))
from attachments import MAX_FILE, identify, clean_name

MAX_BYTES = 512 * 1024 * 1024


def accept(root, event, audio=None):
    identity = event.get('id', '')
    if not re.fullmatch(r'[0-9a-f]{64}', identity):
        raise ValueError('Invalid id')
    if event.get('schemaVersion') != 1 or event.get('type') not in ('kakao', 'sms', 'recording', 'file', 'contacts'):
        raise ValueError('Unsupported event')
    if event['type'] == 'recording':
        if audio is None or len(audio) != event.get('size'):
            raise ValueError('Recording size mismatch')
    elif audio is not None:
        raise ValueError('Unexpected audio')
    event = dict(event)
    images = []
    attachments = event.get('attachments', [])
    if not isinstance(attachments, list) or len(attachments) > 100:
        raise ValueError('Invalid attachments')
    clean = []
    for item in attachments:
        item = dict(item)
        encoded = item.pop('data', None)
        item.pop('imageId', None)
        item.pop('localFile', None)
        if encoded is not None:
            if not isinstance(encoded, str) or len(encoded) > 12 * 1024 * 1024:
                raise ValueError('Preview too large')
            content = base64.b64decode(encoded, validate=True)
            if not content or len(content) > 8 * 1024 * 1024:
                raise ValueError('Invalid preview size')
            image_id = hashlib.sha256(content).hexdigest()
            images.append((image_id, content))
            if len(images) > 10:
                raise ValueError('Too many previews')
            item.update(fileId=image_id, imageId=image_id, size=len(content), mimeType=identify(content[:512],item.get('mimeType','')))
            item.pop('unavailable', None)
        elif item.get('fileId'):
            file_id = item['fileId']
            if not isinstance(file_id,str) or not re.fullmatch('[a-f0-9]{64}',file_id): raise ValueError('Invalid file reference')
            file = root / (file_id + '.blob')
            if not file.is_file() or file.stat().st_size != item.get('size'): raise ValueError('Upload attachment first')
            with file.open('rb') as source: item['mimeType'] = identify(source.read(512),item.get('mimeType',''))
            item.pop('unavailable', None)
        item['fileName'] = clean_name(item.get('fileName'))
        clean.append(item)
    if attachments:
        event['attachments'] = clean
    root.mkdir(parents=True, exist_ok=True)
    with closing(sqlite3.connect(root / 'events.sqlite3')) as db, db:
        db.execute('CREATE TABLE IF NOT EXISTS events(id TEXT PRIMARY KEY, payload TEXT NOT NULL, audio TEXT)')
        if not db.execute('SELECT 1 FROM events WHERE id=?', (identity,)).fetchone():
            for image_id, content in images:
                with tempfile.NamedTemporaryFile(dir=root, delete=False) as tmp:
                    tmp.write(content)
                    tmp.flush()
                    os.fsync(tmp.fileno())
                    temporary = tmp.name
                os.replace(temporary, root / (image_id + '.image'))
            filename = None
            if audio is not None:
                filename = identity + '.audio'
                with tempfile.NamedTemporaryFile(dir=root, delete=False) as tmp:
                    tmp.write(audio)
                    tmp.flush()
                    os.fsync(tmp.fileno())
                    temporary = tmp.name
                os.replace(temporary, root / filename)
            db.execute('INSERT INTO events VALUES (?,?,?)', (identity, json.dumps(event, ensure_ascii=False), filename))
    return {'accepted': True, 'id': identity}


class Handler(BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass  # No message bodies, filenames, or credentials in access logs.

    def reply(self, code, value):
        body = json.dumps(value).encode()
        self.send_response(code)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Content-Length', str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        if self.path == '/health':
            self.reply(200, {'ok': True, 'service': 'PocketRelay'})
        elif self.path == '/app.apk' and getattr(self.server, 'apk_path', None):
            apk = self.server.apk_path
            self.send_response(200)
            self.send_header('Content-Type', 'application/vnd.android.package-archive')
            self.send_header('Content-Disposition', 'attachment; filename="PocketRelay-debug.apk"')
            self.send_header('Content-Length', str(apk.stat().st_size))
            self.end_headers()
            with apk.open('rb') as source:
                import shutil
                shutil.copyfileobj(source, self.wfile)
        else:
            self.reply(404, {'error': 'Not found'})

    def body(self):
        if self.headers.get('Transfer-Encoding', '').lower() == 'chunked':
            data = bytearray()
            while True:
                line = self.rfile.readline(128)
                size = int(line.split(b';')[0].strip(), 16)
                if size == 0:
                    if self.rfile.readline(4) != b'\r\n':
                        raise ValueError('Unexpected trailer')
                    return bytes(data)
                if size < 0 or len(data) + size > MAX_BYTES:
                    raise ValueError('Body too large')
                chunk = self.rfile.read(size)
                if len(chunk) != size or self.rfile.read(2) != b'\r\n':
                    raise ValueError('Incomplete body')
                data.extend(chunk)
        size = int(self.headers.get('Content-Length', '-1'))
        if not 0 <= size <= MAX_BYTES:
            raise ValueError('Invalid body size')
        data = self.rfile.read(size)
        if len(data) != size:
            raise ValueError('Incomplete body')
        return data

    def do_POST(self):
        self.connection.settimeout(90)
        if not hmac.compare_digest(self.headers.get('Authorization', ''), 'Bearer ' + self.server.token):
            self.reply(401, {'error': 'Unauthorized'})
            return
        if self.path.startswith('/v1/attachments/'):
            self.upload_attachment()
            return
        if self.path not in ('/v1/events', '/v1/recordings'):
            self.reply(404, {'error': 'Unknown endpoint'})
            return
        try:
            raw = self.body()
            if self.path == '/v1/events':
                event, audio = json.loads(raw), None
                if event.get('type') not in ('kakao', 'sms', 'file', 'contacts'):
                    raise ValueError('Wrong event type')
            else:
                header = self.headers.get('Content-Type', '')
                msg = BytesParser(policy=default).parsebytes(('Content-Type: ' + header + '\r\nMIME-Version: 1.0\r\n\r\n').encode() + raw)
                parts = {p.get_param('name', header='content-disposition'): p.get_payload(decode=True) for p in msg.iter_parts()}
                event, audio = json.loads(parts['metadata']), parts['file']
                if event.get('type') != 'recording':
                    raise ValueError('Wrong event type')
            if self.headers.get('Idempotency-Key') != event.get('id'):
                raise ValueError('Idempotency key mismatch')
            self.reply(200, accept(self.server.data_root, event, audio))
        except (ValueError, KeyError, TypeError):
            self.reply(400, {'error': 'Invalid payload'})
        except Exception:
            self.reply(500, {'error': 'Storage failed; retry later'})

    def upload_attachment(self):
        temporary = None
        try:
            identity = self.path.removeprefix('/v1/attachments/')
            size = int(self.headers.get('Content-Length','-1'))
            if not re.fullmatch('[a-f0-9]{64}',identity) or self.headers.get('Idempotency-Key') != identity or not 0 <= size <= MAX_FILE:
                raise ValueError('Invalid attachment')
            root = self.server.data_root
            root.mkdir(parents=True,exist_ok=True)
            digest = hashlib.sha256()
            with tempfile.NamedTemporaryFile(dir=root,delete=False) as out:
                temporary = Path(out.name)
                left = size
                while left:
                    data = self.rfile.read(min(left,65536))
                    if not data: raise ValueError('Incomplete attachment')
                    out.write(data);digest.update(data);left -= len(data)
                out.flush();os.fsync(out.fileno())
            if digest.hexdigest() != identity: raise ValueError('Checksum mismatch')
            os.replace(temporary,root/(identity+'.blob'))
            self.reply(200,{'accepted':True,'id':identity})
        except (ValueError,TypeError): self.reply(400,{'error':'Invalid attachment'})
        except Exception: self.reply(500,{'error':'Attachment storage failed'})
        finally:
            if temporary is not None: temporary.unlink(missing_ok=True)


if __name__ == '__main__':
    token = os.environ.get('RELAY_TOKEN', '')
    if len(token) < 16:
        raise SystemExit('Set RELAY_TOKEN to at least 16 random characters.')
    server = HTTPServer(('127.0.0.1', int(os.environ.get('RELAY_PORT', '8787'))), Handler)
    server.token = token
    server.data_root = Path(os.environ.get('RELAY_DATA', 'receiver-data'))
    server.apk_path = Path(__file__).resolve().parents[2] / 'dist' / 'PocketRelay-debug.apk'
    print('Local reference receiver: 127.0.0.1:8787. HTTPS proxy required for Android.')
    server.serve_forever()
