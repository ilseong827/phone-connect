import json
import sqlite3
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch
import chat_viewer

class ChatApiTest(unittest.TestCase):
    def test_empty_database(self):
        with tempfile.TemporaryDirectory() as folder, patch.object(chat_viewer, 'DATA', Path(folder)):
            self.assertEqual(chat_viewer.events(), {'events': [], 'cursor': 0, 'more': False})

    def test_incremental_pagination_no_dropped_events(self):
        with tempfile.TemporaryDirectory() as folder, patch.object(chat_viewer, 'DATA', Path(folder)):
            db = sqlite3.connect(Path(folder) / 'events.sqlite3')
            db.execute('CREATE TABLE events(payload TEXT)')
            db.executemany('INSERT INTO events VALUES (?)', [(json.dumps({'id':str(n),'type':'kakao','message':'<script>test</script>'}),) for n in range(503)])
            db.commit(); db.close()
            first = chat_viewer.events()
            self.assertEqual(len(first['events']), 500)
            self.assertTrue(first['more'])
            second = chat_viewer.events(first['cursor'])
            self.assertEqual(len(second['events']), 3)
            self.assertFalse(second['more'])
            self.assertEqual(len({e['id'] for e in first['events']+second['events']}),503)
            self.assertEqual(chat_viewer.events(second['cursor'])['events'],[])

if __name__ == '__main__':
    unittest.main()
