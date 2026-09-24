#!/usr/bin/env python3
"""Explicit single-task protocol probe. --self-test is offline."""

import io
import json
import os
import re
import time
import unittest
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


def run_probe(args, env=None, opener=None):
    if args != LIVE_FLAGS:
        return 2, "usage: --live (or --self-test offline)"
    if env is None:
        env = os.environ
    key = env.get("GRSAI_KEY", "").strip()
    base = env.get("GRSAI_BASE", "").strip()
    try:
        parsed = urllib.parse.urlsplit(base)
        port = parsed.port
    except ValueError:
        return 2, "configuration_error: invalid GRSAI_BASE"
    if (not key or not base or parsed.scheme != "https" or parsed.hostname not in
            ("grsai.com", "api.grsai.com") or port is not None or parsed.username or
            parsed.password or parsed.query or parsed.fragment or parsed.path not in ("", "/")):
        return 2, "configuration_error: require GRSAI_BASE and GRSAI_KEY"
    if opener is None:
        opener = urllib.request.build_opener(NoRedirect()).open
    deadline = time.monotonic() + 180
    post_started = False
    try:
        payload = json.dumps({"model": "nano-banana-2-lite", "replyType": "stream", "prompt": PROMPT}).encode()
        if post_started:
            raise ProbeError("duplicate_post")
        # Mark before opening: even a transport failure may have submitted a task.
        post_started = True
        with request(opener, "POST", base.rstrip("/") + "/v1/api/generate", key, deadline,
                     payload, "text/event-stream") as response:
            content_type = response.headers.get("Content-Type", "")
            if content_type.split(";", 1)[0].strip().lower() != "text/event-stream":
                raise ProbeError("invalid_content_type")
            task_id, status, progress = parse_stream(response, deadline)
        result_url = base.rstrip("/") + "/v1/api/result?" + urllib.parse.urlencode({"id": task_id})
        while True:
            with request(opener, "GET", result_url, key, deadline) as response:
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
    except Exception as exc:
        # Never print exception text, request URLs, response bodies, or the task ID.
        reason = exc.args[0] if isinstance(exc, ProbeError) else "transport_error"
        return 1, "probe_failed=" + reason


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


class FakeResponse(io.BytesIO):
    def __init__(self, body, content_type):
        super().__init__(body)
        self.headers = {"Content-Type": content_type}
        self.status = 200


def valid_env():
    return {"GRSAI_BASE": "https://api.grsai.com", "GRSAI_KEY": "secret-key"}


class ProbeTests(unittest.TestCase):
    def test_default_and_invalid_flags_never_open_network(self):
        # @covers AC-008
        for args in ([], ["--live", "--max-credits", "10000"], ["--self-test", "--live"]):
            calls = []
            code, output = run_probe(args, valid_env(), lambda req, timeout: calls.append(req))
            self.assertEqual(2, code)
            self.assertIn("--live", output)
            self.assertEqual([], calls)

    def test_bad_environment_never_open_network(self):
        for env in ({}, {"GRSAI_BASE": "https://api.grsai.com"},
                    {**valid_env(), "GRSAI_BASE": "https://evil.example"},
                    {**valid_env(), "GRSAI_BASE": "https://api.grsai.com:bad"}):
            calls = []
            code, output = run_probe(["--live"], env,
                                     lambda req, timeout: calls.append(req))
            self.assertEqual(2, code)
            self.assertEqual([], calls)
            self.assertNotIn("secret-key", output)

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


if __name__ == "__main__":
    import sys
    if sys.argv[1:] == ["--self-test"]:
        unittest.main(argv=[sys.argv[0]])
    else:
        code, message = run_probe(sys.argv[1:])
        print(message)
        sys.exit(code)
