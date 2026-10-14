"""Check the real HTTPS edge after each production traffic change, in Compose."""
import json
import os
import urllib.error
import urllib.request

base = 'https://www.gkotta.bid'
release = 'sha-411025133ebf4728eba0700bc4af959c128c1df9'
epoch = os.environ['DOCSIFY_STAGE_EPOCH']
percent = int(os.environ['DOCSIFY_STAGE_PERCENT'])

def get(path, cookie=None):
    headers = {'User-Agent': 'DocsifyProductionAcceptance/1.0'}
    if cookie:
        headers['Cookie'] = f'sub2api_docsify={epoch}-{cookie}'
    try:
        response = urllib.request.urlopen(urllib.request.Request(base + path, headers=headers), timeout=30)
    except urllib.error.HTTPError as error:
        response = error
    return response.status, response.headers, response.read().decode('utf-8')

for cookie in ('candidate', 'stable', None):
    status, headers, body = get('/docs/quick-start', cookie)
    assert status == 200, (cookie, status)
    expected = release if cookie == 'candidate' or cookie is None and percent == 100 else 'legacy'
    if cookie is not None or percent == 100:
        assert headers.get('X-Docs-Release') == expected, (cookie, dict(headers))
        assert ('docsify-4.13.1.min.js' in body) == (expected == release)
    print('PASS article edge route', cookie or 'no-cookie', headers.get('X-Docs-Release'))

for path in ('/docs/index.md', '/docs/_assets/config.js', '/docs/logo.svg'):
    status, headers, _ = get(path, 'candidate')
    assert status == 200, (path, status)
    assert headers.get('X-Docs-Release') == release, (path, dict(headers))
print('PASS raw Markdown, runtime and logo edge routes')

status, _, body = get('/health')
assert status == 200 and json.loads(body)['status'] == 'ok'
status, _, body = get('/auth/login')
assert status == 200 and '<script' in body and 'docsify-4.13.1.min.js' not in body
print('PASS main application health and login')
print('PASS production stage', percent, 'epoch', epoch)
