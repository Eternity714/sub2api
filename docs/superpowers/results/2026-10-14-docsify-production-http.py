"""Read-only production checks, executed by the Compose Python service."""
import os
import runpy
import unittest
from urllib.parse import urlsplit

checks = runpy.run_path('/workspace/deploy/tests/docsify-http-test.py')
checks['HTTP'].addheaders = [('User-Agent', 'DocsifyProductionAcceptance/1.0')]
original_script_parser = checks['Scripts'].handle_starttag


def production_script_parser(self, tag, attributes):
    attrs = dict(attributes)
    src = urlsplit(attrs.get('src', ''))
    # The existing Cloudflare edge appends optional telemetry to the HTML.
    # Docsify CSP blocks it; the browser suite verifies no external requests.
    # Keep the container suite strict and recognize only this named edge beacon.
    if (tag == 'script' and src.scheme == 'https'
            and src.netloc == 'static.cloudflareinsights.com'
            and src.path.startswith('/beacon.min.js/')
            and 'data-cf-beacon' in attrs):
        return
    original_script_parser(self, tag, attributes)


checks['Scripts'].handle_starttag = production_script_parser
get = checks['get']
suite = unittest.TestSuite()
for name in unittest.defaultTestLoader.getTestCaseNames(checks['DocsifyHTTPTest']):
    if name not in ('test_mounted_markdown_update_is_visible_without_restart',
                    'test_maintenance_and_hidden_files_are_private'):
        suite.addTest(checks['DocsifyHTTPTest'](name))


class ProductionBoundary(unittest.TestCase):
    def test_promoted_routes_ignore_old_and_candidate_cookies(self):
        epoch = os.environ.get('DOCSIFY_PROMOTED_EPOCH')
        if not epoch:
            self.skipTest('Only check all-cookie routing after promotion')
        for cookie in (f'{int(epoch) - 1}-stable', f'{epoch}-stable', f'{epoch}-candidate'):
            with self.subTest(cookie=cookie):
                request_headers = {'Cookie': f'sub2api_docsify={cookie}'}
                status, headers, body = get('/docs/', request_headers)
                self.assertEqual(status, 200)
                self.assertEqual(headers.get('X-Docs-Release'), os.environ['DOCSIFY_EXPECTED_RELEASE'])
                self.assertIn('docsify-4.13.1.min.js', body)
                probe = os.environ.get('DOCSIFY_PROMOTED_PROBE')
                if probe:
                    # A canonical-only file distinguishes slots sharing one image.
                    status, _, _ = get(probe, request_headers)
                    self.assertEqual(status, 200)
                    status, _, text = get(probe + '.md', request_headers)
                    self.assertEqual(status, 200)
                    self.assertIn(os.environ['DOCSIFY_PROMOTED_PROBE_TEXT'], text)
                status, headers, _ = get('/docs/assets/no-such-legacy.js', request_headers)
                self.assertEqual(status, 404)
                self.assertEqual(headers.get('X-Docs-Release'), os.environ['DOCSIFY_EXPECTED_RELEASE'])

    def test_private_paths_and_old_assets_have_real_404(self):
        for path in ('/docs/README', '/docs/README.md', '/docs/.vitepress/config.ts',
                     '/docs/.git/config', '/docs/assets/no-such-legacy.js'):
            with self.subTest(path=path):
                self.assertEqual(get(path)[0], 404)

    def test_docs_release_header_matches_immutable_image(self):
        release = os.environ.get('DOCSIFY_EXPECTED_RELEASE')
        if not release:
            self.skipTest('Direct zero-traffic container has no edge release header')
        for path in ('/docs/', '/docs/quick-start', '/docs/index.md',
                     '/docs/_assets/config.js', '/docs/logo.svg'):
            with self.subTest(path=path):
                status, headers, _ = get(path)
                self.assertEqual(status, 200)
                self.assertEqual(headers.get('X-Docs-Release'), release)
                self.assertIn("script-src 'self';", headers.get('Content-Security-Policy', ''))

    def test_main_application_health_and_login(self):
        if not os.environ.get('DOCSIFY_EXPECTED_RELEASE'):
            self.skipTest('Direct candidate has no main application upstream')
        status, headers, body = get('/health')
        self.assertEqual(status, 200)
        self.assertIn('application/json', headers.get('Content-Type', ''))
        self.assertIn('"status":"ok"', body.replace(' ', ''))
        status, _, body = get('/auth/login')
        self.assertEqual(status, 200)
        scripts = checks['Scripts']()
        scripts.feed(body)
        self.assertTrue(scripts.sources)
        self.assertFalse(any('docsify' in path.lower() for path in scripts.sources))


suite.addTests(unittest.defaultTestLoader.loadTestsFromTestCase(ProductionBoundary))
print('Read-only production target:', os.environ['DOCSIFY_TEST_URL'], flush=True)
result = unittest.TextTestRunner(verbosity=2).run(suite)
raise SystemExit(0 if result.wasSuccessful() else 1)
