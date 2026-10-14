"""Check the real HTTPS edge after each production traffic change, in Compose."""
import json
import os
import urllib.error
import urllib.request

base = os.environ.get('DOCSIFY_TEST_URL', 'https://www.gkotta.bid').rstrip('/')
release = os.environ.get('DOCSIFY_EXPECTED_RELEASE', 'sha-411025133ebf4728eba0700bc4af959c128c1df9')
stable_release = os.environ.get('DOCSIFY_STABLE_RELEASE', 'legacy')
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
    candidate_route = percent != 0 and (cookie == 'candidate' or cookie is None and percent == 100)
    expected = release if candidate_route else stable_release
    if cookie is not None or percent in (0, 100):
        assert headers.get('X-Docs-Release') == expected, (cookie, dict(headers))
        assert ('docsify-4.13.1.min.js' in body) == (expected != 'legacy')
        expected_route = 'candidate' if candidate_route else 'stable'
        assert f'sub2api_docsify={epoch}-{expected_route}' in headers.get('Set-Cookie', ''), (cookie, dict(headers))
    print('PASS article edge route', cookie or 'no-cookie', headers.get('X-Docs-Release'))

for path in ('/docs/index.md', '/docs/_assets/config.js', '/docs/logo.svg'):
    status, headers, _ = get(path, 'candidate')
    assert status == 200, (path, status)
    content_release = stable_release if stable_release != 'legacy' and percent == 0 else release
    assert headers.get('X-Docs-Release') == content_release, (path, dict(headers))
print('PASS raw Markdown, runtime and logo edge routes')

probe = os.environ.get('DOCSIFY_STAGE_CANDIDATE_PROBE')
if probe:
    assert probe.startswith('/docs/') and not any(c in probe for c in '?#')
    for cookie in ('candidate', 'stable', None):
        if cookie is None and percent not in (0, 100):
            continue
        candidate_route = percent != 0 and (cookie == 'candidate' or cookie is None and percent == 100)
        for path in (probe, probe + '.md'):
            status, _, body = get(path, cookie)
            assert status == (200 if candidate_route else 404), (cookie, path, status)
            if candidate_route and path.endswith('.md'):
                assert os.environ['DOCSIFY_STAGE_PROBE_TEXT'] in body, (cookie, path, body)
    print('PASS canonical-only article and raw Markdown prove actual candidate routing')

status, _, body = get('/health')
assert status == 200 and json.loads(body)['status'] == 'ok'
status, _, body = get('/auth/login')
assert status == 200 and '<script' in body and 'docsify-4.13.1.min.js' not in body
print('PASS main application health and login')
print('PASS production stage', percent, 'epoch', epoch)
