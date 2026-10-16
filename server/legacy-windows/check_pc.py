"""Exercise the active HTTPS tunnel with synthetic data, then remove only those fixtures."""
import hashlib
import io
import json
import sqlite3
import urllib.request
import uuid
import wave
from contextlib import closing
from pathlib import Path

root = Path(__file__).resolve().parents[2]
config = json.loads((root / '.tools/pc-session.json').read_text('utf-8-sig'))
ids = []


def post(path, data, content_type, identity):
    req = urllib.request.Request(config['url'] + path, data=data, headers={
        'Authorization': 'Bearer ' + config['token'], 'Idempotency-Key': identity,
        'Content-Type': content_type})
    with urllib.request.urlopen(req, timeout=30) as response:
        ack = json.load(response)
        assert ack == {'accepted': True, 'id': identity}


try:
    identity = hashlib.sha256(uuid.uuid4().bytes).hexdigest()
    ids.append(identity)
    event = {'schemaVersion': 1, 'id': identity, 'type': 'kakao', 'sender': 'TEST',
             'chatRoom': 'TEST', 'message': 'Synthetic connectivity test', 'receivedAt': 0}
    data = json.dumps(event).encode()
    post('/v1/events', data, 'application/json', identity)
    post('/v1/events', data, 'application/json', identity)
    audio = io.BytesIO()
    with wave.open(audio, 'wb') as f:
        f.setnchannels(1)
        f.setsampwidth(2)
        f.setframerate(8000)
        f.writeframes(b'\x00' * 3200)
    audio = audio.getvalue()
    identity = hashlib.sha256(uuid.uuid4().bytes).hexdigest()
    ids.append(identity)
    meta = {'schemaVersion': 1, 'id': identity, 'type': 'recording', 'fileName': 'synthetic-test.wav',
            'mimeType': 'audio/wav', 'size': len(audio), 'fileModifiedAt': 0}
    boundary = 'RelayTest'
    data = ('--'+boundary+'\r\nContent-Disposition: form-data; name="metadata"\r\n\r\n'+json.dumps(meta)+'\r\n--'+boundary+'\r\nContent-Disposition: form-data; name="file"; filename="test.wav"\r\n\r\n').encode()+audio+('\r\n--'+boundary+'--\r\n').encode()
    post('/v1/recordings', data, 'multipart/form-data; boundary='+boundary, identity)
    assert (root / 'receiver-data' / (identity+'.audio')).read_bytes() == audio
    with closing(sqlite3.connect(root / 'receiver-data/events.sqlite3')) as db:
        for identity in ids:
            assert db.execute('SELECT count(*) FROM events WHERE id=?', (identity,)).fetchone()[0] == 1
    req = urllib.request.Request(config['url']+'/v1/events', data=b'{}', headers={'Content-Type':'application/json'})
    try:
        urllib.request.urlopen(req, timeout=20)
        raise AssertionError('Unauthenticated upload accepted')
    except urllib.error.HTTPError as error:
        assert error.code == 401
    with urllib.request.urlopen(config['url']+'/app.apk', timeout=30) as response:
        assert hashlib.sha256(response.read()).digest() == hashlib.sha256((root/'dist/PocketRelay-debug.apk').read_bytes()).digest()
    print('PASS: HTTPS message upload, recording bytes, duplicate retry, auth rejection, APK download checksum')
finally:
    db_file = root/'receiver-data/events.sqlite3'
    if db_file.exists():
        with closing(sqlite3.connect(db_file)) as db, db:
            for identity in ids:
                db.execute('DELETE FROM events WHERE id=?', (identity,))
                path = root/'receiver-data'/(identity+'.audio')
                if path.exists():
                    path.unlink()
