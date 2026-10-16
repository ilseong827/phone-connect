"""File classification is for preview only; arbitrary files are downloaded."""
import re

MAX_FILE = 100 * 1024 * 1024
PREVIEW_IMAGES = {'image/jpeg', 'image/png', 'image/gif', 'image/webp'}
PREVIEW_MEDIA = {'video/mp4', 'video/webm', 'audio/mpeg', 'audio/ogg', 'audio/wav'}

def identify(head, declared='application/octet-stream'):
    if head.startswith(b'\xff\xd8\xff'): return 'image/jpeg'
    if head.startswith(b'\x89PNG\r\n\x1a\n'): return 'image/png'
    if head.startswith((b'GIF87a', b'GIF89a')): return 'image/gif'
    if head[:4] == b'RIFF' and head[8:12] == b'WEBP': return 'image/webp'
    if head[:4] == b'RIFF' and head[8:12] == b'WAVE': return 'audio/wav'
    if head.startswith(b'%PDF-'): return 'application/pdf'
    if head[4:8] == b'ftyp':
        if head[8:12] in (b'heic',b'heix',b'hevc',b'hevx',b'mif1',b'msf1'): return 'image/heic'
        return 'video/mp4'
    if head.startswith(b'OggS'): return 'audio/ogg'
    if head.startswith(b'ID3'): return 'audio/mpeg'
    if head.startswith(b'\x1aE\xdf\xa3') and b'webm' in head: return 'video/webm'
    # Do not preview a claimed media type whose signature was not recognized.
    if declared.startswith(('image/','video/','audio/')): return 'application/octet-stream'
    return declared if re.fullmatch(r'[a-zA-Z0-9.+_-]+/[a-zA-Z0-9.+_-]+', declared) else 'application/octet-stream'

def clean_name(value):
    return re.sub(r'[\x00-\x1f\x7f/\\]', '_', str(value or '첨부파일'))[:200]
