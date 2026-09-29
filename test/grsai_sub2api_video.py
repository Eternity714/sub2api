"""GRSAI-VIDEO-08: one paid generation via Sub2API; resumable polling and MP4 download."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import time
import urllib.error
import urllib.parse
import urllib.request


def load_env(path):
    if path:
        for line in Path(path).read_text(encoding="utf-8-sig").splitlines():
            name, sep, value = line.strip().partition("=")
            if sep and name and not name.startswith("#"):
                os.environ.setdefault(name, value.strip().strip("\"'"))


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *args, **kwargs):
        raise RuntimeError("API redirect refused; verify base URL")


def api(base, key, path, body=None):
    req = urllib.request.Request(base + path, data=None if body is None else json.dumps(body).encode(),
        headers={"Authorization": "Bearer " + key, "Content-Type": "application/json"},
        method="GET" if body is None else "POST")
    try:
        with urllib.request.build_opener(NoRedirect).open(req, timeout=60) as response:
            return response.status, json.load(response)
    except urllib.error.HTTPError as exc:
        # Never print response bodies that might contain secrets or signed URLs.
        raise RuntimeError(f"Sub2API HTTP {exc.code}") from None


def run(args):
    load_env(args.env_file)
    base = (args.base or os.environ.get("SUB2API_BASE") or os.environ.get("SUB2API_BASE_URL", "")).rstrip("/")
    key = os.environ.get("SUB2API_KEY") or os.environ.get("SUB2API_USER_API_KEY", "")
    parsed = urllib.parse.urlparse(base)
    if parsed.scheme != "https" and not (parsed.scheme == "http" and parsed.hostname in ("127.0.0.1", "localhost")):
        raise RuntimeError("Use HTTPS or a local SSH tunnel")
    if not key:
        raise RuntimeError("Set SUB2API_KEY or pass --env-file")
    output = Path(args.output)
    output.mkdir(parents=True, exist_ok=True)
    task_id = args.task_id
    if not task_id:
        code, created = api(base, key, "/v1/api/generate", {
            "model": args.model, "duration": args.duration, "resolution": args.resolution,
            "aspectRatio": args.aspect_ratio,
            "prompt": args.prompt, "replyType": "async"})
        if code != 202 or not created.get("id"):
            raise RuntimeError("Expected async task acceptance")
        task_id = created["id"]
        (output / "task.json").write_text(json.dumps({"id": task_id}), encoding="utf-8")
        print("Task accepted; id saved to task.json. POST is never retried.", flush=True)
    deadline = time.monotonic() + args.timeout
    while time.monotonic() < deadline:
        _, task = api(base, key, "/v1/api/result?id=" + urllib.parse.quote(task_id, safe=""))
        state = task.get("status")
        print(f"status={state}", flush=True)
        if state in ("failed", "manual_review"):
            raise RuntimeError("Task terminal status: " + state)
        if state == "succeeded":
            urls = [item["url"] for item in task.get("result", {}).get("results", [])]
            if not urls:
                raise RuntimeError("No result URLs")
            files = []
            for index, url in enumerate(urls):
                if urllib.parse.urlparse(url).scheme != "https":
                    raise RuntimeError("Result must be HTTPS")
                target = output / f"video-{index}.mp4"
                size = 0
                digest = hashlib.sha256()
                # No API key is sent to media storage.
                with urllib.request.urlopen(url, timeout=120) as stream, target.open("wb") as out:
                    prefix = stream.read(32)
                    if len(prefix) < 12 or prefix[4:8] != b"ftyp":
                        raise RuntimeError("Result is not MP4")
                    while prefix:
                        size += len(prefix)
                        if size > 512 * 1024 * 1024:
                            raise RuntimeError("Video exceeds 512 MiB")
                        digest.update(prefix)
                        out.write(prefix)
                        prefix = stream.read(1024 * 1024)
                files.append({"path": str(target), "bytes": size, "sha256": digest.hexdigest()})
            summary = {"status": "succeeded", "result_count": len(files), "files": files}
            (output / "summary.json").write_text(json.dumps(summary, indent=2), encoding="utf-8")
            print(json.dumps(summary), flush=True)
            return
        time.sleep(args.interval)
    raise RuntimeError("Polling timeout; resume with --task-id, do not resubmit")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base")
    parser.add_argument("--env-file")
    parser.add_argument("--model", default="minimax-h3")
    parser.add_argument("--duration", type=int, default=1)
    parser.add_argument("--resolution", default="480p")
    parser.add_argument("--aspect-ratio", choices=("portrait", "landscape", "square"), default="landscape")
    parser.add_argument("--prompt", default="A cinematic ocean wave at sunrise, smooth camera movement.")
    parser.add_argument("--task-id", help="Resume an existing task without another paid POST")
    parser.add_argument("--output", default="test/output/grsai-video")
    parser.add_argument("--timeout", type=int, default=1200)
    parser.add_argument("--interval", type=float, default=10)
    args = parser.parse_args()
    try:
        run(args)
    except (RuntimeError, urllib.error.URLError, TimeoutError) as error:
        print(type(error).__name__ + ": " + (str(error) if isinstance(error, RuntimeError) else "network operation failed"), flush=True)
        raise SystemExit(1)
