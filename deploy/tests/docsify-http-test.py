#!/usr/bin/env python3
"""Exercise the real Docsify HTTP boundary; run through Docker Compose."""

import os
import re
import unittest
import uuid
from html.parser import HTMLParser
from pathlib import Path
from urllib.error import HTTPError
from urllib.parse import urljoin, urlsplit
from urllib.request import HTTPRedirectHandler, Request, build_opener


BASE_URL = os.environ.get("DOCSIFY_TEST_URL", "http://host.docker.internal:18083").rstrip("/")
DOCS_DIR = Path(os.environ.get("DOCSIFY_TEST_CONTENT_DIR", "/workspace/frontend/docs"))


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, request, response, code, message, headers, new_url):
        return None


HTTP = build_opener(NoRedirect)


def get(path, headers=None, base_url=BASE_URL):
    request = Request(base_url + path, headers=headers or {})
    try:
        response = HTTP.open(request, timeout=15)
    except HTTPError as error:
        response = error
    with response:
        return response.status, response.headers, response.read().decode("utf-8", errors="replace")


class Scripts(HTMLParser):
    def __init__(self):
        super().__init__()
        self.sources = []
        self.has_app = False

    def handle_starttag(self, tag, attributes):
        attrs = dict(attributes)
        if attrs.get("id") == "app":
            self.has_app = True
        if tag == "script" and attrs.get("src"):
            self.sources.append(attrs["src"])


class DocsifyHTTPTest(unittest.TestCase):
    def assert_shell(self, path):
        status, headers, body = get(path)
        self.assertEqual(status, 200, path)
        self.assertIn("text/html", headers.get("Content-Type", ""), path)
        scripts = Scripts()
        scripts.feed(body)
        self.assertTrue(scripts.has_app, f"{path} has no document mount")
        self.assertTrue(any("docsify" in urlsplit(src).path.lower() for src in scripts.sources),
                        f"{path} does not load the Docsify runtime")
        for src in scripts.sources:
            resolved = urlsplit(urljoin(BASE_URL + path, src))
            self.assertEqual(resolved.netloc, urlsplit(BASE_URL).netloc,
                             f"runtime script must be local: {src}")
        self.assert_no_persistent_cache(headers, path)
        return scripts

    def assert_no_persistent_cache(self, headers, path):
        self.assertIn("no-store", headers.get("Cache-Control", "").lower(),
                      f"{path} can persist stale document content")

    def test_entry_and_nested_articles_load_docsify(self):
        for path in ("/docs/", "/docs/quick-start", "/docs/scenarios/", "/docs/scenarios/cherry-studio"):
            with self.subTest(path=path):
                self.assert_shell(path)

    def test_docs_redirect_preserves_query(self):
        status, headers, _ = get("/docs?from=console&term=a%20b")
        self.assertEqual(status, 308)
        target = urlsplit(urljoin(BASE_URL, headers.get("Location", "")))
        self.assertEqual(target.path, "/docs/")
        self.assertEqual(target.query, "from=console&term=a%20b")

    def test_historical_batch_image_redirect_preserves_query(self):
        for path in ("/docs/batch-image", "/docs/batch-image/"):
            with self.subTest(path=path):
                status, headers, _ = get(path + "?group=12&prompt=a%20b")
                self.assertEqual(status, 308, path)
                target = urlsplit(urljoin(BASE_URL, headers.get("Location", "")))
                self.assertEqual(target.path, "/batch-image")
                self.assertEqual(target.query, "group=12&prompt=a%20b")

    def test_sidebar_articles_and_raw_markdown_are_public(self):
        status, headers, sidebar = get("/docs/_sidebar.md")
        self.assertEqual(status, 200)
        self.assert_no_persistent_cache(headers, "/docs/_sidebar.md")
        self.assertIn("快速开始", sidebar)
        self.assertNotIn("<html", sidebar.lower())
        links = set(re.findall(r"\[[^\]]+\]\((/docs/[^\s)]*)\)", sidebar))
        self.assertGreaterEqual(len(links), 35, "sidebar lost existing tutorials")
        self.assertTrue({"/docs/quick-start", "/docs/scenarios/", "/docs/scenarios/cherry-studio"} <= links)
        for link in sorted(links):
            path = urlsplit(link).path
            with self.subTest(path=path):
                self.assert_shell(path)
                markdown_path = path + "index.md" if path.endswith("/") else path + ".md"
                status, headers, markdown = get(markdown_path)
                self.assertEqual(status, 200, markdown_path)
                self.assertNotIn("text/html", headers.get("Content-Type", ""), markdown_path)
                self.assertNotIn("<html", markdown.lower(), markdown_path)
                self.assertTrue(markdown.strip(), markdown_path)
                self.assert_no_persistent_cache(headers, markdown_path)

    def test_navigation_files_do_not_persist(self):
        for path in ("/docs/index.md", "/docs/_navbar.md", "/docs/_404.md"):
            with self.subTest(path=path):
                status, headers, body = get(path)
                self.assertEqual(status, 200, path)
                self.assertTrue(body.strip(), path)
                self.assert_no_persistent_cache(headers, path)

    def test_missing_articles_and_assets_return_real_404(self):
        for path in ("/docs/no-such-docsify-article", "/docs/scenarios/no-such-docsify-article/",
                     "/docs/no-such-docsify-article.md", "/docs/no-such-docsify-script.js",
                     "/docs/no-such-docsify-image.png", "/docs/_assets/no-such-runtime.js"):
            with self.subTest(path=path):
                status, _, _ = get(path)
                self.assertEqual(status, 404, f"{path} fell back to an application shell")

    def test_maintenance_and_hidden_files_are_private(self):
        for path in ("/docs/README", "/docs/README.md", "/docs/.vitepress/config.ts",
                     "/docs/.git/config", "/docs/.private/index.md", "/docs/.private/"):
            with self.subTest(path=path):
                status, _, _ = get(path)
                self.assertEqual(status, 404, path)
        hidden = DOCS_DIR / (".docsify-private-test-" + uuid.uuid4().hex)
        try:
            hidden.mkdir()
            (hidden / "index.md").write_text("# Private maintenance content\n", encoding="utf-8")
            for path in (f"/docs/{hidden.name}/", f"/docs/{hidden.name}/index.md"):
                with self.subTest(path=path):
                    self.assertEqual(get(path)[0], 404, "an existing hidden file became public")
        finally:
            (hidden / "index.md").unlink(missing_ok=True)
            hidden.rmdir()

    def test_mounted_markdown_update_is_visible_without_restart(self):
        name = "docsify-live-test-" + uuid.uuid4().hex
        fixture = DOCS_DIR / (name + ".md")
        path = "/docs/" + name + ".md"
        old = "# Docsify live test\n\nFirst live content.\n"
        new = "# Docsify live test\n\nUpdated mounted content.\n"
        try:
            fixture.write_text(old, encoding="utf-8")
            status, headers, body = get(path)
            self.assertEqual(status, 200)
            self.assertEqual(body, old)
            self.assert_no_persistent_cache(headers, path)
            fixture.write_text(new, encoding="utf-8")
            status, headers, body = get(path, {"If-None-Match": headers.get("ETag", '"first-read"'),
                                              "If-Modified-Since": headers.get("Last-Modified", "Wed, 01 Jan 2020 00:00:00 GMT")})
            self.assertEqual(status, 200, "modified Markdown must not receive a stale 304")
            self.assertEqual(body, new)
            self.assert_no_persistent_cache(headers, path)
        finally:
            fixture.unlink(missing_ok=True)
        self.assertEqual(get(path)[0], 404, "deleted mounted Markdown must disappear immediately")


class DocsifyEdgeHTTPTest(unittest.TestCase):
    """Check the real same-origin edge against the existing local main app."""

    main_url = os.environ.get("DOCSIFY_TEST_MAIN_URL", "http://host.docker.internal:18081").rstrip("/")

    def test_health_stays_on_the_main_application(self):
        direct_status, direct_headers, direct_body = get("/health", base_url=self.main_url)
        edge_status, edge_headers, edge_body = get("/health")
        self.assertEqual(direct_status, 200, "the local main application is unhealthy")
        self.assertEqual(edge_status, direct_status)
        self.assertIn("application/json", edge_headers.get("Content-Type", ""),
                      "edge /health reached Docsify's plain-text health endpoint")
        self.assertEqual(edge_headers.get("Content-Type"), direct_headers.get("Content-Type"))
        self.assertEqual(edge_body.strip(), direct_body.strip())

    def test_main_home_and_login_keep_main_application_assets(self):
        for path in ("/", "/auth/login"):
            with self.subTest(path=path):
                direct_status, _, direct_body = get(path, base_url=self.main_url)
                edge_status, headers, edge_body = get(path)
                self.assertEqual(direct_status, 200, f"main application {path}")
                self.assertEqual(edge_status, direct_status, path)
                self.assertIn("text/html", headers.get("Content-Type", ""), path)
                direct_scripts = Scripts()
                direct_scripts.feed(direct_body)
                edge_scripts = Scripts()
                edge_scripts.feed(edge_body)
                self.assertTrue(direct_scripts.sources, "main HTML contains no application scripts")
                self.assertEqual(edge_scripts.sources, direct_scripts.sources,
                                 "the edge changed the main application's asset references")
                self.assertFalse(any("docsify" in src.lower() for src in edge_scripts.sources),
                                 "the main application route reached the document runtime")


if __name__ == "__main__":
    print(f"Docsify HTTP target: {BASE_URL}", flush=True)
    suite = unittest.defaultTestLoader.loadTestsFromTestCase(DocsifyHTTPTest)
    if os.environ.get("DOCSIFY_TEST_EDGE") == "1":
        suite.addTests(unittest.defaultTestLoader.loadTestsFromTestCase(DocsifyEdgeHTTPTest))
    result = unittest.TextTestRunner(verbosity=2).run(suite)
    raise SystemExit(0 if result.wasSuccessful() else 1)
