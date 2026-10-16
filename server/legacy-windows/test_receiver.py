import hashlib
import base64
import http.client
import json
import sqlite3
import tempfile
import threading
import unittest
from contextlib import closing
from pathlib import Path
from http.server import HTTPServer
from receiver import Handler, accept


class ReceiverTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.root = Path(self.tmp.name)
        self.event = {'id': hashlib.sha256(b'event').hexdigest(), 'schemaVersion': 1, 'type': 'kakao',
                      'sender': '발신자', 'chatRoom': '채팅방', 'message': '안녕하세요', 'receivedAt': 1}

    def tearDown(self):
        self.tmp.cleanup()

    def test_retry_is_idempotent(self):
        self.assertEqual(accept(self.root, self.event), accept(self.root, self.event))
        with closing(sqlite3.connect(self.root / 'events.sqlite3')) as db:
            self.assertEqual(db.execute('SELECT count(*) FROM events').fetchone()[0], 1)

    def test_recording_bytes_and_metadata(self):
        self.event.update(type='recording', size=4, fileName='통화 녹음.m4a')
        result = accept(self.root, self.event, b'test')
        self.assertTrue(result['accepted'])
        self.assertEqual((self.root / (self.event['id'] + '.audio')).read_bytes(), b'test')
        with self.assertRaises(ValueError):
            accept(self.root, self.event, b'bad')

    def test_path_traversal_rejected(self):
        self.event['id'] = '../../outside'
        with self.assertRaises(ValueError):
            accept(self.root, self.event)

    def test_sent_mms_image_and_retry(self):
        content = b'\x89PNG\r\n\x1a\nfixture'
        self.event.update(type='sms', direction='outgoing', protocol='MMS', attachments=[{
            'mimeType': 'image/png', 'data': base64.b64encode(content).decode(), 'imageId': '../../bad'}])
        accept(self.root, self.event)
        accept(self.root, self.event)
        with closing(sqlite3.connect(self.root / 'events.sqlite3')) as db:
            rows = db.execute('SELECT payload FROM events').fetchall()
        self.assertEqual(len(rows), 1)
        saved = json.loads(rows[0][0])
        self.assertEqual(saved['direction'], 'outgoing')
        item = saved['attachments'][0]
        self.assertNotIn('data', item)
        self.assertEqual((self.root / (item['imageId'] + '.image')).read_bytes(), content)

    def test_bad_image_rejected_without_event(self):
        self.event['attachments'] = [{'mimeType':'image/png', 'data':'not base64!'}]
        with self.assertRaises(ValueError):
            accept(self.root, self.event)
        self.assertFalse((self.root / 'events.sqlite3').exists())

    def test_svg_not_served_as_image(self):
        self.event['attachments'] = [{'mimeType':'image/svg+xml', 'data':base64.b64encode(b'<svg/>').decode()}]
        accept(self.root, self.event)
        with closing(sqlite3.connect(self.root / 'events.sqlite3')) as db:
            item = json.loads(db.execute('SELECT payload FROM events').fetchone()[0])['attachments'][0]
        self.assertEqual(item['mimeType'], 'application/octet-stream')
        self.assertIn('fileId', item)

    def test_incomplete_image_mime_detected_from_bytes(self):
        self.event['attachments'] = [{'mimeType':'image/', 'data':base64.b64encode(b'GIF89a'+b'\0'*20).decode()}]
        accept(self.root, self.event)
        with closing(sqlite3.connect(self.root / 'events.sqlite3')) as db:
            item = json.loads(db.execute('SELECT payload FROM events').fetchone()[0])['attachments'][0]
        self.assertEqual(item['mimeType'],'image/gif')
        self.assertNotIn('unavailable',item)

    def test_http_auth_and_chunked_upload(self):
        srv = HTTPServer(('127.0.0.1', 0), Handler)
        srv.token, srv.data_root = 'test-token-1234567890', self.root
        t = threading.Thread(target=srv.serve_forever, daemon=True)
        t.start()
        try:
            conn = http.client.HTTPConnection(*srv.server_address)
            conn.request('POST', '/v1/events', b'{}')
            response = conn.getresponse()
            self.assertEqual(response.status, 401)
            response.read()
            conn.close()
            self.event.update(type='recording', size=4, fileName='녹음.m4a')
            boundary = 'TestBoundary'
            body = ('--'+boundary+'\r\nContent-Disposition: form-data; name="metadata"\r\n\r\n'+json.dumps(self.event)+'\r\n--'+boundary+'\r\nContent-Disposition: form-data; name="file"; filename="recording.bin"\r\n\r\ntest\r\n--'+boundary+'--\r\n').encode()
            headers = {'Authorization': 'Bearer '+srv.token, 'Content-Type': 'multipart/form-data; boundary='+boundary,
                       'Idempotency-Key': self.event['id']}
            conn = http.client.HTTPConnection(*srv.server_address)
            conn.request('POST', '/v1/recordings', iter([body[:50], body[50:]]), headers, encode_chunked=True)
            response = conn.getresponse()
            self.assertEqual(response.status, 200)
            self.assertEqual(json.loads(response.read())['id'], self.event['id'])
            conn.close()
        finally:
            srv.shutdown()
            srv.server_close()
            t.join()


if __name__ == '__main__':
    unittest.main()
