import http.client,json,sqlite3,tempfile,threading,unittest
from pathlib import Path
from unittest.mock import patch
from http.server import ThreadingHTTPServer
import local_viewer
import wave
from audio_preview import prepare
from contextlib import closing
from chat_viewer import ChatViewer

class AudioTest(unittest.TestCase):
    def test_pcm_playback_conversion_and_cache(self):
        if not (Path(__file__).resolve().parent.parent/'.tools/ffmpeg.exe').is_file():self.skipTest('FFmpeg not installed')
        with tempfile.TemporaryDirectory() as directory:
            source=Path(directory)/'sample.audio'
            with wave.open(str(source),'wb') as f:f.setnchannels(1);f.setsampwidth(2);f.setframerate(48000);f.writeframes(b'\0\0'*4800)
            original=source.read_bytes();result=prepare(source)
            with wave.open(str(result),'rb') as f:self.assertEqual(f.getnframes(),4800);self.assertEqual(f.getframerate(),48000)
            timestamp=result.stat().st_mtime_ns;self.assertEqual(prepare(source).stat().st_mtime_ns,timestamp);self.assertEqual(source.read_bytes(),original)
    def test_mislabeled_aac_range_and_download(self):
        with tempfile.TemporaryDirectory() as directory,patch.object(local_viewer,'DATA',Path(directory)):
            root=Path(directory);identity='a'*64;data=b'\0\0\0\x18ftyp3gp4'+bytes(range(64))
            (root/(identity+'.audio')).write_bytes(data)
            with closing(sqlite3.connect(root/'events.sqlite3')) as db,db:
                db.execute('CREATE TABLE events(id TEXT,payload TEXT)')
                db.execute('INSERT INTO events VALUES (?,?)',(identity,json.dumps({'mimeType':'audio/mpeg','fileName':'record.m4a'})))
            server=ThreadingHTTPServer(('127.0.0.1',0),ChatViewer);thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start()
            try:
                for range_value,code,expected in [('bytes=0-15',206,data[:16]),('bytes=-4',206,data[-4:]),('bytes=9999-',416,None),(None,200,data)]:
                    conn=http.client.HTTPConnection(*server.server_address);headers={'Host':'127.0.0.1:8788'}
                    if range_value:headers['Range']=range_value
                    conn.request('GET','/audio/'+identity,headers=headers);r=conn.getresponse();self.assertEqual(r.status,code)
                    body=r.read()
                    if expected is not None:self.assertEqual(body,expected);self.assertEqual(r.getheader('Content-Type'),'audio/mp4');self.assertEqual(r.getheader('Accept-Ranges'),'bytes')
                    conn.close()
                conn=http.client.HTTPConnection(*server.server_address);conn.request('GET','/audio/'+identity+'?download=1',headers={'Host':'127.0.0.1:8788'});r=conn.getresponse();self.assertEqual(r.status,200);self.assertTrue(r.getheader('Content-Disposition').startswith('attachment'));self.assertEqual(r.read(),data);conn.close()
            finally:server.shutdown();server.server_close();thread.join()
