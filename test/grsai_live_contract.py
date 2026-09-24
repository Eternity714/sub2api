#!/usr/bin/env python3
"""Explicit single-task protocol probe. --self-test is offline."""

import io
import json
import os
import re
import time
import unittest
import urllib.error
import urllib.parse
import urllib.request
from unittest.mock import patch


PROMPT = "A red square on a white background."
LIVE_FLAGS = ["--live"]
TERMINAL = {"succeeded", "failed", "violation"}
STATUSES = {"pending": "running", "queued": "running", "processing": "running",
            "in_progress": "running", "running": "running", "success": "succeeded",
            "completed": "succeeded", "complete": "succeeded", "succeeded": "succeeded",
            "failure": "failed", "error": "failed", "failed": "failed",
            "blocked": "violation", "moderation": "violation", "violation": "violation",
            "input_moderation": "violation", "output_moderation": "violation"}


class ProbeError(Exception):
    pass


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def scrub(value, key=""):
    if key:
        value = value.replace(key, "[redacted]")
    value = re.sub(r"Bearer\s+\S+", "Bearer [redacted]", value, flags=re.I)
    value = re.sub(r"prompt=\S+", "prompt=[redacted]", value, flags=re.I)
    return re.sub(r"https?://\S+", "[redacted-url]", value, flags=re.I)


def fields(value):
    if not isinstance(value, dict):
        raise ProbeError("invalid_json")
    data = value.get("data")
    return value, data if isinstance(data, dict) else {}


def scalar(root, data, *names):
    for obj in (root, data):
        for name in names:
            value = obj.get(name)
            if isinstance(value, (str, int)) and not isinstance(value, bool) and str(value).strip():
                return str(value).strip()
    return ""


def event(raw, prior_id="", prior_progress=0):
    try:
        root, data = fields(json.loads(raw))
    except (ValueError, UnicodeError) as exc:
        raise ProbeError("invalid_json") from exc
    task_id = scalar(root, data, "id", "taskId", "task_id")
    if not task_id or (prior_id and task_id != prior_id):
        raise ProbeError("unstable_id")
    status = STATUSES.get(scalar(root, data, "status", "state").lower() or "running")
    if status is None:
        raise ProbeError("invalid_status")
    progress = prior_progress
    for obj in (root, data):
        for name in ("progress", "percent", "percentage"):
            if name in obj:
                value = obj[name]
                if isinstance(value, bool) or not isinstance(value, (int, float)) or int(value) != value:
                    raise ProbeError("invalid_progress")
                progress = int(value)
                break
        else:
            continue
        break
    if progress < 0 or progress < prior_progress or progress > 100:
        raise ProbeError("invalid_progress")
    if status == "succeeded" and not any(root.get(k) or data.get(k) for k in
                                         ("results", "result", "result_urls", "resultURLs", "images", "urls")):
        raise ProbeError("missing_result")
    return task_id, status, progress


def parse_stream(response, deadline):
    task_id, progress, terminal = "", 0, ""
    data = []
    frames = 0
    while True:
        if time.monotonic() >= deadline:
            raise ProbeError("timeout")
        line = response.readline(1 << 20)
        if len(line) >= 1 << 20:
            raise ProbeError("frame_too_large")
        if not line:
            if data:
                if terminal in TERMINAL:
                    raise ProbeError("event_after_terminal")
                task_id, terminal, progress = event(b"\n".join(data), task_id, progress)
                frames += 1
            break
        line = line.rstrip(b"\r\n")
        if line.startswith(b"data:"):
            data.append(line[5:].lstrip(b" "))
            if sum(map(len, data)) > 1 << 20:
                raise ProbeError("frame_too_large")
        elif not line and data:
            if terminal in TERMINAL:
                raise ProbeError("event_after_terminal")
            task_id, terminal, progress = event(b"\n".join(data), task_id, progress)
            frames += 1
            data = []
        if frames >= 10000:
            raise ProbeError("too_many_frames")
    if terminal not in TERMINAL:
        raise ProbeError("missing_terminal")
    return task_id, terminal, progress


def request(opener, method, url, key, deadline, body=None, accept="application/json"):
    remaining = deadline - time.monotonic()
    if remaining <= 0:
        raise ProbeError("timeout")
    req = urllib.request.Request(url, data=body, method=method,
                                 headers={"Authorization": "Bearer " + key, "Accept": accept})
    if body is not None:
        req.add_header("Content-Type", "application/json")
    response = opener(req, timeout=remaining)
    if response.status < 200 or response.status >= 300:
        response.close()
        raise ProbeError("http_status")
    return response


def live_credentials(env):
    key = env.get("GRSAI_KEY", "").strip()
    base = env.get("GRSAI_BASE", "").strip()
    try:
        parsed = urllib.parse.urlsplit(base)
        port = parsed.port
    except ValueError:
        return None
    if (not key or not base or parsed.scheme != "https" or parsed.hostname !=
            "grsaiapi.com" or port is not None or parsed.username or
            parsed.password or parsed.query or parsed.fragment or parsed.path not in ("", "/")):
        return None
    return base.rstrip("/"), key


def run_probe(args, env=None, opener=None):
    if args != LIVE_FLAGS:
        return 2, "usage: --live (or --self-test offline)"
    credentials = live_credentials(os.environ if env is None else env)
    if credentials is None:
        return 2, "configuration_error: require GRSAI_BASE and GRSAI_KEY"
    base, key = credentials
    if opener is None:
        opener = urllib.request.build_opener(NoRedirect()).open
    deadline = time.monotonic() + 180
    post_started = False
    phase = "generate_post"
    try:
        payload = json.dumps({"model": "nano-banana-2-lite", "replyType": "stream", "prompt": PROMPT}).encode()
        if post_started:
            raise ProbeError("duplicate_post")
        # Mark before opening: even a transport failure may have submitted a task.
        post_started = True
        with request(opener, "POST", base.rstrip("/") + "/v1/api/generate", key, deadline,
                     payload, "text/event-stream") as response:
            phase = "stream_read"
            content_type = response.headers.get("Content-Type", "")
            if content_type.split(";", 1)[0].strip().lower() != "text/event-stream":
                raise ProbeError("invalid_content_type")
            task_id, status, progress = parse_stream(response, deadline)
        result_url = base.rstrip("/") + "/v1/api/result?" + urllib.parse.urlencode({"id": task_id})
        while True:
            phase = "result_get"
            with request(opener, "GET", result_url, key, deadline) as response:
                phase = "result_parse"
                if response.headers.get("Content-Type", "").split(";", 1)[0].strip().lower() != "application/json":
                    raise ProbeError("invalid_result_content_type")
                raw = response.read(1 << 20)
                if len(raw) >= 1 << 20:
                    raise ProbeError("result_too_large")
                result_id, result_status, _ = event_result(raw)
            if result_id != task_id:
                raise ProbeError("result_id_mismatch")
            if result_status == "running":
                remaining = deadline - time.monotonic()
                if remaining <= 0:
                    raise ProbeError("timeout")
                time.sleep(min(1, remaining))
                continue
            if result_status != status or status != "succeeded":
                raise ProbeError("result_status_mismatch")
            break
        suffix = "".join(c if c.isascii() and (c.isalnum() or c in "-_") else "?"
                         for c in task_id[-4:])
        return 0, "protocol=passed task_suffix=***" + suffix + f" content_type=text/event-stream status={status} progress={progress}"
    except urllib.error.HTTPError as exc:
        exc.close()
        return 1, f"probe_failed={phase}_http_status_{exc.code}"
    except urllib.error.URLError as exc:
        reason = "transport_timeout" if isinstance(exc.reason, TimeoutError) else "transport_error"
        return 1, f"probe_failed={phase}_{reason}"
    except Exception as exc:
        # Never print exception text, request URLs, response bodies, or the task ID.
        reason = exc.args[0] if isinstance(exc, ProbeError) else "transport_error"
        return 1, f"probe_failed={phase}_{reason}"


def event_result(raw):
    try:
        root, data = fields(json.loads(raw))
    except (ValueError, UnicodeError) as exc:
        raise ProbeError("invalid_result_json") from exc
    task_id = scalar(root, data, "id", "taskId", "task_id")
    status = STATUSES.get(scalar(root, data, "status", "state").lower())
    if not task_id or status is None:
        raise ProbeError("invalid_result")
    return task_id, status, None


def compare_mode(mode, base, key, opener):
    deadline = time.monotonic() + 180
    summary = {"generate_http": None, "generate_status": None,
               "result_http": None, "result_status": None, "result_id_match": None}
    payload = json.dumps({"model": "nano-banana-2-lite", "replyType": mode, "prompt": PROMPT}).encode()
    phase = "generate_post"
    try:
        with request(opener, "POST", base + "/v1/api/generate", key, deadline,
                     payload, "text/event-stream" if mode == "stream" else "application/json") as response:
            summary["generate_http"] = response.status
            phase = "generate_read"
            if mode == "stream":
                if response.headers.get("Content-Type", "").split(";", 1)[0].strip().lower() != "text/event-stream":
                    raise ProbeError("invalid_content_type")
                task_id, summary["generate_status"], _ = parse_stream(response, deadline)
            else:
                if response.headers.get("Content-Type", "").split(";", 1)[0].strip().lower() != "application/json":
                    raise ProbeError("invalid_content_type")
                body = response.read(1 << 20)
                if len(body) >= 1 << 20:
                    raise ProbeError("response_too_large")
                task_id, summary["generate_status"], _ = event_result(body)
        url = base + "/v1/api/result?" + urllib.parse.urlencode({"id": task_id})
        summary["result_http_history"] = []
        for attempt in range(3):
            phase = "result_get"
            try:
                with request(opener, "GET", url, key, deadline) as response:
                    summary["result_http"] = response.status
                    summary["result_http_history"].append(response.status)
                    phase = "result_read"
                    if response.headers.get("Content-Type", "").split(";", 1)[0].strip().lower() != "application/json":
                        raise ProbeError("invalid_content_type")
                    body = response.read(1 << 20)
                    if len(body) >= 1 << 20:
                        raise ProbeError("response_too_large")
                    result_id, summary["result_status"], _ = event_result(body)
                    summary["result_id_match"] = result_id == task_id
                    break
            except urllib.error.HTTPError as exc:
                summary["result_http"] = exc.code
                summary["result_http_history"].append(exc.code)
                exc.close()
                if exc.code != 404 or attempt == 2:
                    break
                time.sleep(min(5, max(0, deadline - time.monotonic())))
    except urllib.error.HTTPError as exc:
        if phase == "generate_post":
            summary["generate_http"] = exc.code
        elif phase == "result_get":
            summary["result_http"] = exc.code
        else:
            summary["error"] = phase + "_http_error"
        exc.close()
    except (urllib.error.URLError, TimeoutError):
        summary["error"] = phase + "_transport_error"
    except Exception as exc:
        summary["error"] = phase + "_" + (exc.args[0] if isinstance(exc, ProbeError) else "transport_error")
    return summary


def run_mode_comparison(args, env=None, opener=None):
    if args != ["--compare-modes"]:
        return 2, "usage: --compare-modes"
    credentials = live_credentials(os.environ if env is None else env)
    if credentials is None:
        return 2, "configuration_error: require GRSAI_BASE and GRSAI_KEY"
    base, key = credentials
    if opener is None:
        opener = urllib.request.build_opener(NoRedirect()).open
    results = {}
    for mode in ("stream", "async", "json"):
        results[mode] = compare_mode(mode, base, key, opener)
        # A transport error during POST may still have submitted a task. Stop
        # instead of creating more tasks while the outcome is unknown.
        if results[mode].get("error") in ("generate_post_transport_error", "generate_read_transport_error"):
            break
    complete = len(results) == 3 and all(
        item["generate_http"] == 200 and item["result_http"] is not None and "error" not in item
        for item in results.values())
    return 0 if complete else 1, json.dumps(results, separators=(",", ":"))


class FakeResponse(io.BytesIO):
    def __init__(self, body, content_type):
        super().__init__(body)
        self.headers = {"Content-Type": content_type}
        self.status = 200


def valid_env():
    return {"GRSAI_BASE": "https://grsaiapi.com", "GRSAI_KEY": "secret-key"}


class ProbeTests(unittest.TestCase):
    def test_mode_comparison_checks_each_generated_id_once_without_leaking_data(self):
        stream = b'data: {"id":"stream-private","status":"succeeded","results":["https://images.example/private.png"]}\n\n'
        replies = [FakeResponse(stream, "text/event-stream"),
                   FakeResponse(b'{"id":"async-private","status":"running"}', "application/json"),
                   FakeResponse(b'{"id":"async-private","status":"running"}', "application/json"),
                   FakeResponse(b'{"id":"json-private","status":"succeeded"}', "application/json")]
        calls = []

        def opener(req, timeout):
            calls.append(req)
            if req.get_method() == "GET" and ("stream-private" in req.full_url or "json-private" in req.full_url):
                raise urllib.error.HTTPError(req.full_url, 404, "private", {}, io.BytesIO(b"private"))
            return replies.pop(0)

        with patch("time.sleep"):
            code, output = run_mode_comparison(["--compare-modes"], valid_env(), opener)
        self.assertEqual(0, code)
        self.assertEqual(["POST", "GET", "GET", "GET", "POST", "GET", "POST", "GET", "GET", "GET"],
                         [req.get_method() for req in calls])
        self.assertEqual(["stream", "async", "json"],
                         [json.loads(req.data)["replyType"] for req in calls if req.get_method() == "POST"])
        results = json.loads(output)
        self.assertEqual(404, results["stream"]["result_http"])
        self.assertEqual([404, 404, 404], results["stream"]["result_http_history"])
        self.assertEqual(200, results["async"]["result_http"])
        self.assertTrue(results["async"]["result_id_match"])
        self.assertEqual(404, results["json"]["result_http"])
        for private in ("secret-key", "private", "red square", "images.example"):
            self.assertNotIn(private, output)

    def test_mode_comparison_requires_explicit_flag_and_credentials(self):
        calls = []
        opener = lambda req, timeout: calls.append(req)
        self.assertEqual(2, run_mode_comparison([], valid_env(), opener)[0])
        self.assertEqual(2, run_mode_comparison(["--compare-modes"], {}, opener)[0])
        self.assertEqual([], calls)

    def test_mode_comparison_stops_after_uncertain_post_transport(self):
        calls = []

        def failing(req, timeout):
            calls.append(req)
            raise urllib.error.URLError(TimeoutError("private"))

        code, output = run_mode_comparison(["--compare-modes"], valid_env(), failing)
        self.assertEqual(1, code)
        self.assertEqual(["POST"], [req.get_method() for req in calls])
        self.assertEqual("generate_post_transport_error", json.loads(output)["stream"]["error"])
        self.assertNotIn("private", output)

    def test_default_and_invalid_flags_never_open_network(self):
        # @covers AC-008
        for args in ([], ["--live", "--max-credits", "10000"], ["--self-test", "--live"]):
            calls = []
            code, output = run_probe(args, valid_env(), lambda req, timeout: calls.append(req))
            self.assertEqual(2, code)
            self.assertIn("--live", output)
            self.assertEqual([], calls)

    def test_bad_environment_never_open_network(self):
        for env in ({}, {"GRSAI_BASE": "https://grsaiapi.com"},
                    {**valid_env(), "GRSAI_BASE": "https://evil.example"},
                    {**valid_env(), "GRSAI_BASE": "https://api.grsai.com"},
                    {**valid_env(), "GRSAI_BASE": "https://grsaiapi.com:bad"}):
            calls = []
            code, output = run_probe(["--live"], env,
                                     lambda req, timeout: calls.append(req))
            self.assertEqual(2, code)
            self.assertEqual([], calls)
            self.assertNotIn("secret-key", output)

    def test_global_base_uses_provider_endpoints(self):
        stream = b'data: {"id":"task-1","status":"succeeded","results":[{"url":"https://images.example/1.png"}]}\n\n'
        replies = [FakeResponse(stream, "text/event-stream"),
                   FakeResponse(b'{"id":"task-1","status":"succeeded"}', "application/json")]
        urls = []

        def opener(req, timeout):
            urls.append(req.full_url)
            return replies.pop(0)

        code, _ = run_probe(["--live"], valid_env(), opener)
        self.assertEqual(0, code)
        self.assertEqual(["https://grsaiapi.com/v1/api/generate",
                          "https://grsaiapi.com/v1/api/result?id=task-1"], urls)

    def test_scrubber(self):
        output = scrub("Bearer secret-key prompt=private https://images.example/a.png", "secret-key")
        for private in ("secret-key", "private", "https://images.example/a.png"):
            self.assertNotIn(private, output)

    def test_single_post_stream_and_result(self):
        stream = b'data: {"id":"task-123456","status":"running","progress":5}\n\ndata: {"id":"task-123456","status":"succeeded","progress":100,"results":["https://images.example/private.png"]}\n\n'
        replies = [FakeResponse(stream, "text/event-stream; charset=utf-8"), FakeResponse(b'{"data":{"id":"task-123456","status":"succeeded"}}', "application/json")]
        calls = []

        def opener(req, timeout):
            calls.append(req)
            return replies.pop(0)

        code, output = run_probe(["--live"], valid_env(), opener)
        self.assertEqual(0, code)
        self.assertEqual(["POST", "GET"], [req.get_method() for req in calls])
        self.assertEqual("stream", json.loads(calls[0].data)["replyType"])
        self.assertIn("protocol=passed", output)
        self.assertNotIn("credit", output)
        for private in ("task-123456", "private.png", "secret-key", "red square"):
            self.assertNotIn(private, output)

    def test_untrusted_id_cannot_inject_output_lines(self):
        task_id = "a\n\nb"
        frame = json.dumps({"id": task_id, "status": "succeeded", "results": ["https://images.example/private.png"]})
        replies = [FakeResponse(("data: " + frame + "\n\n").encode(), "text/event-stream"),
                   FakeResponse(json.dumps({"id": task_id, "status": "succeeded"}).encode(), "application/json")]
        code, output = run_probe(["--live"], valid_env(),
                                 lambda req, timeout: replies.pop(0))
        self.assertEqual(0, code)
        self.assertNotIn("\n", output)

    def test_rejects_bad_stream_without_retry(self):
        for body in (b'{"private":"prompt"}',
                     b'data: {"id":"task-123456","progress":10}\n\ndata: {"id":"other","progress":20}\n\n',
                     b'data: {"id":"task-123456","progress":10}\n\ndata: {"id":"task-123456","progress":1}\n\n'):
            calls = []
            kind = "application/json" if body.startswith(b'{') else "text/event-stream"
            code, output = run_probe(["--live"], valid_env(),
                                     lambda req, timeout: calls.append(req) or FakeResponse(body, kind))
            self.assertEqual(1, code)
            self.assertEqual(1, len(calls))
            self.assertNotIn("task-123456", output)
            self.assertNotIn("private", output)

    def test_rejects_event_after_terminal(self):
        stream = (b'data: {"id":"a","status":"succeeded","results":["https://images.example/private.png"]}\n\n'
                  b'data: {"id":"a","status":"succeeded","results":["https://images.example/private.png"]}\n\n')
        calls = []
        code, output = run_probe(["--live"], valid_env(),
                                 lambda req, timeout: calls.append(req) or FakeResponse(stream, "text/event-stream"))
        self.assertEqual(1, code)
        self.assertEqual(1, len(calls))
        self.assertNotIn("private.png", output)

    def test_result_mismatch_and_error_are_sanitized(self):
        stream = b'data: {"id":"task-123456","status":"succeeded","progress":100,"results":["https://images.example/private.png"]}\n\n'
        replies = [FakeResponse(stream, "text/event-stream"), FakeResponse(b'{"id":"other","status":"succeeded"}', "application/json")]
        code, _ = run_probe(["--live"], valid_env(), lambda req, timeout: replies.pop(0))
        self.assertEqual(1, code)

        def failing(req, timeout):
            raise ValueError("secret-key https://images.example/private.png")

        code, output = run_probe(["--live"], valid_env(), failing)
        self.assertEqual(1, code)
        self.assertNotIn("secret-key", output)
        self.assertNotIn("private.png", output)

    def test_deadline_prevents_post(self):
        calls = []
        with patch("time.monotonic", side_effect=[0, 181]):
            code, _ = run_probe(["--live"], valid_env(),
                                lambda req, timeout: calls.append(req))
        self.assertEqual(1, code)
        self.assertEqual([], calls)

    def test_http_error_reports_only_status_without_response_body(self):
        def failing(req, timeout):
            raise urllib.error.HTTPError(req.full_url, 403, "secret-key", {}, io.BytesIO(b"secret-key"))

        code, output = run_probe(["--live"], valid_env(), failing)
        self.assertEqual(1, code)
        self.assertEqual("probe_failed=generate_post_http_status_403", output)

    def test_result_http_error_is_distinguished_from_successful_stream(self):
        stream = b'data: {"id":"task-123456","status":"succeeded","results":["https://images.example/private.png"]}\n\n'
        calls = []

        def failing_result(req, timeout):
            calls.append(req)
            if req.get_method() == "POST":
                return FakeResponse(stream, "text/event-stream")
            raise urllib.error.HTTPError(req.full_url, 404, "private", {}, io.BytesIO(b"private"))

        code, output = run_probe(["--live"], valid_env(), failing_result)
        self.assertEqual(1, code)
        self.assertEqual(["POST", "GET"], [req.get_method() for req in calls])
        self.assertEqual("probe_failed=result_get_http_status_404", output)
        for private in ("task-123456", "private.png", "secret-key", "red square"):
            self.assertNotIn(private, output)

    def test_transport_timeout_does_not_expose_error(self):
        def failing(req, timeout):
            raise urllib.error.URLError(TimeoutError("secret-key"))

        code, output = run_probe(["--live"], valid_env(), failing)
        self.assertEqual(1, code)
        self.assertEqual("probe_failed=generate_post_transport_timeout", output)


if __name__ == "__main__":
    import sys
    if sys.argv[1:] == ["--self-test"]:
        unittest.main(argv=[sys.argv[0]])
    else:
        code, message = (run_mode_comparison(sys.argv[1:]) if sys.argv[1:] == ["--compare-modes"]
                         else run_probe(sys.argv[1:]))
        print(message)
        sys.exit(code)
