#!/usr/bin/env python3
"""Outbound-only Phone Connect client. Uses standard HTTP(S) proxy settings."""
import argparse
import json
import os
from pathlib import Path
import re
import sys
import urllib.error
import urllib.parse
import urllib.request

class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None  # Never forward the read key to a redirected destination.

def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--config', required=True, help='private JSON with url and readKey')
    p.add_argument('operation', choices=['relay_status','search_messages','get_conversation','get_record','list_recordings','get_transcript','search_transcripts','download_audio'])
    p.add_argument('--query', default='')
    p.add_argument('--type', choices=['sms','kakao'])
    p.add_argument('--room')
    p.add_argument('--id')
    p.add_argument('--device-id')
    p.add_argument('--after', type=int, default=0)
    p.add_argument('--limit', type=int, default=50)
    p.add_argument('--out', help='new output file for download_audio')
    a = p.parse_args()
    config = json.loads(Path(a.config).read_text(encoding='utf-8-sig'))
    url = config['url'].rstrip('/')
    parsed = urllib.parse.urlsplit(url)
    if parsed.scheme != 'https' or not parsed.hostname or parsed.username or parsed.password or parsed.query or parsed.fragment:
        raise ValueError('Config url must be a plain HTTPS base URL')
    key = config['readKey']
    if len(key) < 32 or any(c.isspace() for c in key):
        raise ValueError('Invalid read key')
    args = {'query': a.query, 'after': a.after, 'limit': a.limit}
    for name, value in [('type',a.type),('room',a.room),('id',a.id),('deviceId',a.device_id)]:
        if value is not None: args[name] = value
    if a.operation == 'download_audio':
        if not a.id or not re.fullmatch('[a-f0-9]{64}', a.id) or not a.out:
            raise ValueError('download_audio requires --id and --out')
        req = urllib.request.Request(url+'/v1/audio/'+a.id)
    else:
        req = urllib.request.Request(url+'/v1/read/'+a.operation, data=json.dumps(args).encode(), headers={'Content-Type':'application/json'})
    req.add_header('Authorization', 'Bearer '+key)
    opener = urllib.request.build_opener(NoRedirect())
    with opener.open(req, timeout=60) as response:
        if a.operation == 'download_audio':
            # Exclusive creation prevents replacing an existing user file.
            with open(a.out, 'xb') as target:
                while chunk := response.read(65536): target.write(chunk)
            print(json.dumps({'downloaded':a.out,'id':a.id}))
        else:
            value = json.load(response)
            print(json.dumps(value, ensure_ascii=False, indent=2))

if __name__ == '__main__':
    try:
        main()
    except urllib.error.HTTPError as e:
        print(f'HTTP {e.code}: request failed (check server URL/read key)', file=sys.stderr)
        sys.exit(1)
    except (OSError, ValueError, KeyError, urllib.error.URLError) as e:
        print(f'{type(e).__name__}: request/config failed; check network and private config', file=sys.stderr)
        sys.exit(1)
