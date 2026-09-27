#!/usr/bin/env python3
"""Opt-in GRS.AI Async contract probe. Never prints credentials or result URLs."""

import argparse
import json
import os
import sys
import time
import urllib.error
import urllib.parse
import urllib.request


def task_fields(payload):
    if not isinstance(payload, dict):
        return "", "", 0
    data = payload.get("data")
    if not isinstance(data, dict):
        data = payload
    task_id = data.get("id") or data.get("taskId") or payload.get("id")
    status = data.get("status") or payload.get("status")
    results = data.get("results") or payload.get("results") or []
    return task_id if isinstance(task_id, str) else "", status if isinstance(status, str) else "", len(results) if isinstance(results, list) else 0


def emit(**fields):
    print(json.dumps(fields, ensure_ascii=True, separators=(",", ":")), flush=True)


def self_test():
    original = urllib.request.urlopen
    def forbidden(*_args, **_kwargs):
        raise AssertionError("self-test attempted network access")
    urllib.request.urlopen = forbidden
    try:
        assert task_fields({"id": "private", "status": "running"}) == ("private", "running", 0)
        assert task_fields({"data": {"id": "private", "status": "succeeded", "results": [{"url": "private"}]}}) == ("private", "succeeded", 1)
    finally:
        urllib.request.urlopen = original
    emit(self_test="passed", network_requests=0)
    return 0


def request_json(url, key, method="GET", body=None):
    data = None if body is None else json.dumps(body, separators=(",", ":")).encode("utf-8")
    request = urllib.request.Request(url, data=data, method=method,
        headers={"Authorization": "Bearer " + key, "Accept": "application/json", "Content-Type": "application/json"})
    with urllib.request.urlopen(request, timeout=60) as response:
        raw = response.read(16 * 1024 * 1024 + 1)
        if len(raw) > 16 * 1024 * 1024:
            raise ValueError("response too large")
        return response.status, json.loads(raw)


def live(model, timeout_seconds, poll_seconds):
    base = os.environ.get("GRSAI_BASE", "").strip().rstrip("/")
    key = os.environ.get("GRSAI_KEY", "").strip()
    if not base.startswith("https://") or not key:
        emit(error="GRSAI_BASE must be HTTPS and GRSAI_KEY must be set")
        return 2
    try:
        status_code, payload = request_json(base + "/v1/api/generate", key, "POST", {
            "model": model, "prompt": "A plain red circle on a white background", "replyType": "async",
        })
        task_id, status, _ = task_fields(payload)
        emit(phase="submit", http_status=status_code, accepted=bool(task_id), status=status)
        if not task_id:
            return 1
        deadline = time.monotonic() + timeout_seconds
        attempt = 0
        while time.monotonic() < deadline:
            time.sleep(poll_seconds)
            attempt += 1
            query = urllib.parse.urlencode({"id": task_id})
            try:
                status_code, payload = request_json(base + "/v1/api/result?" + query, key)
            except urllib.error.HTTPError as exc:
                if exc.code == 404:
                    emit(phase="poll", attempt=attempt, http_status=404, status="not_ready")
                    continue
                emit(phase="poll", attempt=attempt, http_status=exc.code, status="error")
                return 1
            returned_id, status, result_count = task_fields(payload)
            if returned_id and returned_id != task_id:
                emit(phase="poll", error="task identity mismatch")
                return 1
            emit(phase="poll", attempt=attempt, http_status=status_code, status=status)
            if status in ("succeeded", "failed", "violation"):
                emit(phase="terminal", status=status, result_count=result_count)
                return 0 if status == "succeeded" and result_count > 0 else 1
        emit(phase="terminal", status="timeout")
        return 1
    except Exception as exc:
        emit(phase="probe", error=type(exc).__name__)
        return 1


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--self-test", action="store_true")
    parser.add_argument("--live", action="store_true")
    parser.add_argument("--model")
    parser.add_argument("--timeout-seconds", type=int, default=1800)
    parser.add_argument("--poll-seconds", type=int, default=5)
    args = parser.parse_args(argv)
    if args.self_test:
        return self_test()
    if not args.live:
        emit(status="offline", network_requests=0)
        return 0
    if not args.model or args.timeout_seconds <= 0 or args.poll_seconds <= 0:
        parser.error("--live requires --model and positive timeout/poll values")
    return live(args.model, args.timeout_seconds, args.poll_seconds)


if __name__ == "__main__":
    sys.exit(main())
