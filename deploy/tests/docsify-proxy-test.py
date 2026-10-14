#!/usr/bin/env python3
"""Exercise generated host proxy rules with real Nginx and three HTTP fixtures."""
import http.server
import pathlib
import subprocess
import sys
import threading
import time
import urllib.error
import urllib.request


class Backend(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        status = 404 if self.server.slot != "legacy" and self.path.startswith("/docs/assets/old.js") else 200
        body = (self.server.slot + " " + self.path).encode()
        self.send_response(status)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *_):
        pass


def main():
    nginx, config, epoch, stable, candidate, percent = sys.argv[1:]
    servers = []
    process = None
    try:
        for slot, port in [("legacy", 18080), ("blue", 18083), ("green", 18084)]:
            server = http.server.ThreadingHTTPServer(("127.0.0.1", port), Backend)
            server.slot = slot
            servers.append(server)
            threading.Thread(target=server.serve_forever, daemon=True).start()
        process = subprocess.Popen([nginx, "-c", config, "-g", "daemon off;"], stdout=subprocess.DEVNULL, stderr=subprocess.PIPE)

        def request(path, route):
            query = "?probe=keep%2Fquery&value=42"
            req = urllib.request.Request("http://127.0.0.1:18743" + path + query,
                                         headers={"Cookie": f"sub2api_docsify={epoch}-{route}"})
            for attempt in range(30):
                try:
                    response = urllib.request.urlopen(req, timeout=3)
                    break
                except urllib.error.HTTPError as error:
                    response = error
                    break
                except urllib.error.URLError:
                    if process.poll() is not None or attempt == 29:
                        raise
                    time.sleep(0.05)
            with response:
                return response.status, response.read().decode(), response.headers, path + query

        for route in ["stable", "candidate"]:
            selected = stable if percent == "0" else candidate if route == "candidate" else stable
            content = candidate if stable == "legacy" else selected
            for path in ["/docs", "/docs/quick-start", "/docs/scenarios/"]:
                status, body, headers, uri = request(path, route)
                assert (status, body) == (200, selected + " " + uri), (percent, path, route, status, body)
                assert headers.get("Strict-Transport-Security") == "max-age=123456; includeSubDomains", headers
                assert "Path=/docs; Secure; HttpOnly; SameSite=Lax" in headers.get("Set-Cookie", ""), headers
            for path in ["/docs/quick-start.md", "/docs/_sidebar.md", "/docs/_assets/config.js", "/docs/logo.svg"]:
                status, body, _, uri = request(path, route)
                assert (status, body) == (200, content + " " + uri), (percent, path, route, status, body)
            status, body, _, uri = request("/docs/assets/old.js", route)
            expected_slot = "legacy" if stable == "legacy" else content
            expected_status = 200 if stable == "legacy" else 404
            assert (status, body) == (expected_status, expected_slot + " " + uri), (percent, route, status, body)
        print(f"Real proxy verified: stable={stable}, candidate={candidate}, percent={percent}; URI, cookies, headers and asset routes")
    finally:
        if process is not None:
            process.terminate()
            process.wait(timeout=5)
            errors = process.stderr.read().decode()
            if process.returncode not in (0, -15):
                print(errors, file=sys.stderr)
        for server in servers:
            server.shutdown()
            server.server_close()


if __name__ == "__main__":
    main()
