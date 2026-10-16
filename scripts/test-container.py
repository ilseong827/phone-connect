"""Integration test against an isolated running Podman test container, synthetic data only."""
import hashlib
import json
from pathlib import Path
import subprocess
import time
import urllib.error
import urllib.request

NAME = 'phone-connect-test'

def run(*args):
    return subprocess.check_output(['podman', 'exec', NAME, *args], text=True)

def info(kind):
    return json.loads(run('phone-connect-info', kind))

def request(url, key, body=None):
    headers = {'Authorization': 'Bearer '+key}
    if body is not None: headers['Content-Type'] = 'application/json'
    req = urllib.request.Request(url, data=json.dumps(body).encode() if body is not None else None, headers=headers)
    with urllib.request.urlopen(req, timeout=25) as r:
        return json.load(r)

def ready():
    for attempt in range(60):
        try:
            pair = info('pair')
            dot = info('dot')
            if pair['pairLink'] and info('check')['authenticated']:
                status = request(dot['url']+'/v1/read/relay_status', dot['readKey'], {})
                assert status['readOnly']
                return pair, dot
        except (ValueError, KeyError, subprocess.CalledProcessError, urllib.error.URLError, AssertionError):
            pass
        time.sleep(2)
    raise RuntimeError('Public tunnels did not become ready')

pair, dot = ready()
identity = hashlib.sha256(b'phone-connect-container-integration-v12').hexdigest()
event = {'schemaVersion':1,'id':identity,'type':'sms','message':'synthetic-podman-check',
         'address':'01000000000','direction':'received','receivedAt':1792100000000}
req = urllib.request.Request(pair['phoneServer']+'/v1/events', data=json.dumps(event).encode(),
    headers={'Content-Type':'application/json','Authorization':'Bearer '+pair['uploadKey'],'Idempotency-Key':identity})
for _ in range(2):
    with urllib.request.urlopen(req,timeout=30) as r: assert json.load(r)['accepted']
result = request(dot['url']+'/v1/read/search_messages', dot['readKey'], {'query':'synthetic-podman-check'})
assert len(result['events']) == 1 and result['events'][0]['id'] == identity
for base, path, key in [(dot['url'],'/v1/read/relay_status',pair['uploadKey']),
                        (pair['phoneServer'],'/v1/events',dot['readKey'])]:
    try: request(base+path, key, {})
    except urllib.error.HTTPError as e: assert e.code == 401
    else: raise AssertionError('Credential separation failed')
for path in ['/api/connection','/api/dot-connection','/api/control/stop','/mcp']:
    try: request(dot['url']+path, dot['readKey'])
    except urllib.error.HTTPError as e: assert e.code == 404
    else: raise AssertionError('Private route exposed')
print('PASS: external HTTPS upload, authenticated read, duplicate prevention, credential separation, private-route isolation',flush=True)
Path('.tools/container-test-dot.json').write_text(json.dumps(dot),encoding='utf-8')
old_upload,old_read = pair['uploadKey'],dot['readKey']
subprocess.run(['podman','restart',NAME],check=True,stdout=subprocess.DEVNULL)
pair,dot=ready()
assert pair['uploadKey']==old_upload and dot['readKey']==old_read
record=request(dot['url']+'/v1/read/get_record',dot['readKey'],{'id':identity})
assert record['message']=='synthetic-podman-check'
Path('.tools/container-test-dot.json').write_text(json.dumps(dot),encoding='utf-8')
print('PASS: container restart preserves bind-mounted data and both keys',flush=True)
