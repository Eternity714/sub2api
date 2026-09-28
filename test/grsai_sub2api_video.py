import os, json, urllib.request, urllib.error

base = os.environ.get("SUB2API_BASE", "").rstrip("/")
key = os.environ.get("SUB2API_KEY", "")
if not base or not key:
    raise SystemExit("请设置 SUB2API_BASE 和 SUB2API_KEY")
body = {
    "model": os.environ.get("GRSAI_VIDEO_MODEL", "minimax-h3"),
    "prompt": os.environ.get("GRSAI_VIDEO_PROMPT", "A short cinematic ocean wave at sunrise"),
    "duration": int(os.environ.get("GRSAI_VIDEO_DURATION", "5")),
    "resolution": os.environ.get("GRSAI_VIDEO_RESOLUTION", "768p"),
    "replyType": "async",
}
request = urllib.request.Request(
    base + "/v1/api/generate",
    data=json.dumps(body).encode(),
    headers={"Authorization": "Bearer " + key, "Content-Type": "application/json"},
    method="POST",
)
try:
    with urllib.request.urlopen(request, timeout=60) as response:
        print(json.dumps({"status": response.status, "response": json.load(response)}, ensure_ascii=False))
except urllib.error.HTTPError as error:
    print(json.dumps({"status": error.code, "error": error.read().decode(errors="replace")}, ensure_ascii=False))
    raise SystemExit(1)
