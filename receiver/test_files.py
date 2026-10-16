import hashlib,http.client,json,sqlite3,tempfile,threading,unittest
from contextlib import closing
from http.server import ThreadingHTTPServer
from pathlib import Path
from unittest.mock import patch
import chat_viewer
from receiver import Handler,accept

class FilesTest(unittest.TestCase):
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory();self.root=Path(self.tmp.name)
        self.server=ThreadingHTTPServer(('127.0.0.1',0),Handler)
        self.server.token='test-only-secret-token';self.server.data_root=self.root
        self.thread=threading.Thread(target=self.server.serve_forever,daemon=True);self.thread.start()
    def tearDown(self):
        self.server.shutdown();self.server.server_close();self.thread.join();self.tmp.cleanup()
    def upload(self,data,identity=None,authorized=True):
        identity=identity or hashlib.sha256(data).hexdigest()
        conn=http.client.HTTPConnection(*self.server.server_address)
        conn.request('POST','/v1/attachments/'+identity,data,{'Authorization':'Bearer '+(self.server.token if authorized else 'bad'),'Idempotency-Key':identity})
        r=conn.getresponse();status=r.status;r.read();conn.close();return status
    def test_file_types_and_range_download(self):
        fixtures=[('animation.gif',b'GIF89a'+b'1234567890','image/'),('video.mp4',b'\0\0\0\x18ftypmp42'+b'0123456789','video/mp4'),('report.pdf',b'%PDF-1.4\nfixture','application/pdf'),('report.docx',b'PK\x03\x04docx','application/vnd.openxmlformats-officedocument.wordprocessingml.document'),('sheet.xlsx',b'PK\x03\x04xlsx','application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'),('unknown.xyz',b'arbitrary bytes','unknown/')]
        for name,data,mime in fixtures:
            identity=hashlib.sha256(data).hexdigest()
            self.assertEqual(self.upload(data),200);self.assertEqual(self.upload(data),200)
            event={'id':hashlib.sha256(name.encode()).hexdigest(),'schemaVersion':1,'type':'file','attachments':[{'fileId':identity,'size':len(data),'mimeType':mime,'fileName':name}]}
            accept(self.root,event)
            self.assertEqual((self.root/(identity+'.blob')).read_bytes(),data)
        with patch.object(chat_viewer,'DATA',self.root):
            viewer=ThreadingHTTPServer(('127.0.0.1',0),chat_viewer.ChatViewer)
            thread=threading.Thread(target=viewer.serve_forever,daemon=True);thread.start()
            try:
                for name,data,mime in fixtures:
                    identity=hashlib.sha256(data).hexdigest();conn=http.client.HTTPConnection(*viewer.server_address)
                    conn.request('GET','/file/'+identity,headers={'Host':'127.0.0.1:8788'})
                    response=conn.getresponse();self.assertEqual(response.status,200);self.assertTrue(response.getheader('Content-Disposition').startswith('attachment'));self.assertEqual(response.read(),data);conn.close()
                identity=hashlib.sha256(fixtures[1][1]).hexdigest();conn=http.client.HTTPConnection(*viewer.server_address)
                conn.request('GET','/file/'+identity+'?preview=1',headers={'Host':'127.0.0.1:8788','Range':'bytes=4-7'})
                response=conn.getresponse();self.assertEqual(response.status,206);self.assertEqual(response.getheader('Content-Type'),'video/mp4');self.assertEqual(response.read(),b'ftyp');conn.close()
                conn=http.client.HTTPConnection(*viewer.server_address);conn.request('GET','/file/'+identity,headers={'Host':'evil.example'});response=conn.getresponse();self.assertEqual(response.status,403);response.read();conn.close()
            finally:viewer.shutdown();viewer.server_close();thread.join()
    def test_auth_checksum_and_missing_reference(self):
        self.assertEqual(self.upload(b'data',authorized=False),401)
        self.assertEqual(self.upload(b'data',identity='0'*64),400)
        self.assertFalse((self.root/('0'*64+'.blob')).exists())
        with self.assertRaises(ValueError):accept(self.root,{'id':'1'*64,'schemaVersion':1,'type':'file','attachments':[{'fileId':'2'*64,'size':4}]})
    def test_contact_update_retains_sms(self):
        accept(self.root,{'id':'1'*64,'schemaVersion':1,'type':'sms','deviceId':'phone','address':'01012345678','message':'test'})
        for identity,name in [('2'*64,'홍길동'),('3'*64,'새 이름')]:
            accept(self.root,{'id':identity,'schemaVersion':1,'type':'contacts','deviceId':'phone','entries':[{'address':'01012345678','displayName':name}]})
        with patch.object(chat_viewer,'DATA',self.root):
            events=chat_viewer.events()['events'];self.assertEqual(len(events),3);self.assertEqual(events[-1]['entries'][0]['displayName'],'새 이름');self.assertEqual(events[0]['type'],'sms')
