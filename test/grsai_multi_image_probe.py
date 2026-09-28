#!/usr/bin/env python3
"""Probe whether one native GRS.AI generation request returns multiple images.

Offline by default. A live run performs exactly one POST and only polls its task;
no image is downloaded, and credentials, task IDs, URLs, and response bodies are
never printed. A live generation may incur provider charges.
"""

import argparse
import json
import os
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path

MAX_RESPONSE_BYTES = 16 << 20
TERMINAL_FAILURES = {"failed", "failure", "error", "violation", "blocked"}
TERMINAL_SUCCESSES = {"succeeded", "success", "completed", "complete"}
COUNT_FIELDS = ("n", "numImages", "num_images", "imageCount", "image_count")
DEFAULT_PROMPT = "Create two separate images of a simple red circle on a plain white background."


def emit(**fields):
    print(json.dumps(fields, ensure_ascii=False, separators=(",", ":")), flush=True)


def load_config():
    config = {}
    env_path = Path(__file__).resolve().parent / ".env"
    if env_path.is_file():
        for line in env_path.read_text(encoding="utf-8-sig").splitlines():
            line = line.strip()
            if line and not line.startswith("#") and "=" in line:
                name, value = line.split("=", 1)
                config[name.strip()] = value.strip().strip("\"'")
    config.update(os.environ)
    return config


def endpoint(base, path):
    parsed = urllib.parse.urlsplit(base.strip().rstrip("/"))
    if (parsed.scheme != "https" or not parsed.hostname or parsed.username or
            parsed.password or parsed.query or parsed.fragment or
            parsed.path.rstrip("/") not in ("", "/v1")):
        raise ValueError("GRSAI_BASE must be an HTTPS origin, optionally ending in /v1")
    root = urllib.parse.urlunsplit((parsed.scheme, parsed.netloc, "", "", ""))
    return root + path


def task_fields(payload):
    """Return safe task metadata without logging the original response."""
    if not isinstance(payload, dict):
        return "", "", None, 0, 0
    data = payload.get("data")
    if not isinstance(data, dict):
        data = payload
    task_id = data.get("id") or data.get("taskId") or payload.get("id")
    status = data.get("status") or payload.get("status")
    results = data.get("results")
    if results is None:
        results = payload.get("results")
    if not isinstance(results, list):
        return task_id if isinstance(task_id, str) else "", str(status or "").lower(), None, 0, 0
    urls = [item["url"] for item in results if isinstance(item, dict) and
            isinstance(item.get("url"), str) and
            urllib.parse.urlsplit(item["url"]).scheme in ("https", "http") and
            urllib.parse.urlsplit(item["url"]).netloc]
    return (task_id if isinstance(task_id, str) else "", str(status or "").lower(),
            len(results), len(urls), len(set(urls)))


def request_json(url, key, method="GET", body=None, timeout=60):
    data = None if body is None else json.dumps(body, ensure_ascii=False).encode("utf-8")
    request = urllib.request.Request(
        url, data=data, method=method,
        headers={"Authorization": "Bearer " + key, "Accept": "application/json",
                 "Content-Type": "application/json", "User-Agent": "sub2api-grsai-multi-image-probe/1.0"},
    )
    with urllib.request.urlopen(request, timeout=timeout) as response:
        raw = response.read(MAX_RESPONSE_BYTES + 1)
        if len(raw) > MAX_RESPONSE_BYTES:
            raise ValueError("response too large")
        return response.status, json.loads(raw)


def verdict(status, result_count, valid_count, unique_count, requested_count):
    if status in TERMINAL_FAILURES:
        return "generation_failed"
    if status not in TERMINAL_SUCCESSES:
        return "inconclusive"
    if result_count is None:
        return "inconclusive_unrecognized_results"
    if unique_count >= requested_count:
        return "multiple_images_confirmed"
    if valid_count < result_count or unique_count < valid_count:
        return "inconclusive_invalid_or_duplicate_results"
    return "requested_count_not_returned"


def self_test():
    original = urllib.request.urlopen
    urllib.request.urlopen = lambda *_a, **_kw: (_ for _ in ()).throw(
        AssertionError("self-test attempted network access"))
    try:
        two = {"data": {"id": "secret", "status": "succeeded", "results": [
            {"url": "https://example.test/a.png"}, {"url": "https://example.test/b.png"}]}}
        assert task_fields(two) == ("secret", "succeeded", 2, 2, 2)
        assert verdict(*task_fields(two)[1:], 2) == "multiple_images_confirmed"
        assert task_fields({"status": "succeeded", "results": [{"url": "https://example.test/a.png"}]})[2:] == (1, 1, 1)
        duplicate = {"status": "succeeded", "results": [{"url": "https://example.test/a.png"}] * 2}
        assert verdict(*task_fields(duplicate)[1:], 2) == "inconclusive_invalid_or_duplicate_results"
        assert verdict("succeeded", 1, 1, 1, 2) == "requested_count_not_returned"
        assert verdict("running", 0, 0, 0, 2) == "inconclusive"
        assert endpoint("https://grsaiapi.com/v1", "/v1/api/generate") == "https://grsaiapi.com/v1/api/generate"
    finally:
        urllib.request.urlopen = original
    emit(self_test="passed", network_requests=0)
    return 0


def live(args, config):
    key = config.get("GRSAI_KEY", "").strip()
    base = args.base or config.get("GRSAI_BASE") or "https://grsaiapi.com"
    if not key:
        emit(error="GRSAI_KEY is required for a direct upstream test")
        return 2
    try:
        generate_url = endpoint(base, "/v1/api/generate")
        result_url = endpoint(base, "/v1/api/result")
    except ValueError as exc:
        emit(error=str(exc))
        return 2
    body = {"model": args.model, "prompt": args.prompt, "images": [],
            "aspectRatio": args.aspect_ratio, "replyType": "async",
            args.count_field: args.requested_count}
    if args.image_size:
        body["imageSize"] = args.image_size
    try:
        http_status, response = request_json(generate_url, key, "POST", body)
        task_id, status, count, valid, unique = task_fields(response)
        emit(phase="submit", http_status=http_status, accepted=bool(task_id), status=status)
        if status in TERMINAL_SUCCESSES | TERMINAL_FAILURES:
            result = verdict(status, count, valid, unique, args.requested_count)
            emit(phase="terminal", status=status, result_count=count,
                 valid_image_count=valid, unique_image_count=unique, verdict=result)
            return 0 if result == "multiple_images_confirmed" else 1
        if not task_id:
            emit(phase="terminal", verdict="inconclusive_no_task_id")
            return 1
        deadline = time.monotonic() + args.timeout_seconds
        attempt = 0
        while time.monotonic() < deadline:
            time.sleep(min(args.poll_seconds, max(0, deadline - time.monotonic())))
            attempt += 1
            query = urllib.parse.urlencode({"id": task_id})
            try:
                http_status, response = request_json(result_url + "?" + query, key)
            except urllib.error.HTTPError as exc:
                if exc.code == 404:
                    emit(phase="poll", attempt=attempt, http_status=404, status="not_ready")
                    continue
                emit(phase="poll", attempt=attempt, http_status=exc.code, verdict="inconclusive_poll_error")
                return 1
            returned_id, status, count, valid, unique = task_fields(response)
            if returned_id and returned_id != task_id:
                emit(phase="terminal", verdict="inconclusive_task_identity_mismatch")
                return 1
            emit(phase="poll", attempt=attempt, http_status=http_status, status=status)
            if status in TERMINAL_SUCCESSES | TERMINAL_FAILURES:
                result = verdict(status, count, valid, unique, args.requested_count)
                emit(phase="terminal", status=status, result_count=count,
                     valid_image_count=valid, unique_image_count=unique, verdict=result)
                return 0 if result == "multiple_images_confirmed" else 1
        emit(phase="terminal", verdict="inconclusive_timeout")
        return 1
    except urllib.error.HTTPError as exc:
        emit(phase="submit", http_status=exc.code, verdict="request_rejected_or_upstream_error")
        return 1
    except (urllib.error.URLError, TimeoutError, OSError, ValueError, TypeError, json.JSONDecodeError) as exc:
        emit(phase="probe", error=type(exc).__name__, verdict="inconclusive")
        return 1


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--self-test", action="store_true", help="offline parsing and verdict checks")
    parser.add_argument("--live", action="store_true", help="opt in to one potentially billable generation")
    parser.add_argument("--base", help="GRS.AI HTTPS origin; defaults to GRSAI_BASE or global origin")
    parser.add_argument("--model", default="gpt-image-2", help="native GRS.AI image model")
    parser.add_argument("--prompt", default=DEFAULT_PROMPT)
    parser.add_argument("--aspect-ratio", default="1:1")
    parser.add_argument("--image-size", default=None)
    parser.add_argument("--requested-count", type=int, default=2)
    parser.add_argument("--count-field", choices=COUNT_FIELDS, default="n",
                        help="experimental upstream count field; one field per run")
    parser.add_argument("--timeout-seconds", type=int, default=1800)
    parser.add_argument("--poll-seconds", type=int, default=5)
    args = parser.parse_args(argv)
    if args.requested_count < 2 or args.requested_count > 4:
        parser.error("--requested-count must be between 2 and 4")
    if args.timeout_seconds <= 0 or args.poll_seconds <= 0:
        parser.error("timeout and polling interval must be positive")
    if args.self_test:
        return self_test()
    if not args.live:
        emit(status="offline", network_requests=0)
        return 0
    return live(args, load_config())


if __name__ == "__main__":
    sys.exit(main())
