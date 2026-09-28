import importlib.util
import json
import os
from pathlib import Path
from types import SimpleNamespace
import unittest
from unittest import mock


SCRIPT = Path(__file__).with_name("grsai_sub2api_video.py")
SPEC = importlib.util.spec_from_file_location("grsai_sub2api_video", SCRIPT)
video = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(video)


class FakeDownload:
    def __init__(self, payload):
        self.payload = payload
        self.offset = 0

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc, tb):
        return False

    def read(self, size=-1):
        if size < 0:
            size = len(self.payload) - self.offset
        chunk = self.payload[self.offset:self.offset + size]
        self.offset += len(chunk)
        return chunk


def args(output, **overrides):
    values = {
        "base": "https://sub2api.example",
        "env_file": None,
        "model": "minimax-h3",
        "duration": 6,
        "resolution": "768p",
        "prompt": "test prompt",
        "task_id": None,
        "output": str(output),
        "timeout": 30,
        "interval": 0,
    }
    values.update(overrides)
    return SimpleNamespace(**values)


class GrsaiSub2APIVideoTest(unittest.TestCase):
    def run_with_key(self, run_args, api_side_effect, urlopen_side_effect=None):
        with mock.patch.dict(os.environ, {"SUB2API_KEY": "top-secret-key"}, clear=True), \
                mock.patch.object(video, "api", side_effect=api_side_effect) as api_mock, \
                mock.patch.object(video.time, "sleep") as sleep_mock, \
                mock.patch.object(video.urllib.request, "urlopen", side_effect=urlopen_side_effect) as urlopen_mock:
            video.run(run_args)
        return api_mock, sleep_mock, urlopen_mock

    def test_submits_once_polls_until_success_and_downloads_every_mp4(self):
        api_responses = iter([
            (202, {"id": "task-123"}),
            (200, {"status": "queued"}),
            (200, {"status": "running"}),
            (200, {"status": "succeeded", "result": {"results": [
                {"url": "https://media.example/one.mp4"},
                {"url": "https://media.example/two.mp4"},
            ]}}),
        ])
        media = {
            "https://media.example/one.mp4": b"\x00\x00\x00\x18ftypisom-video-one",
            "https://media.example/two.mp4": b"\x00\x00\x00\x18ftypmp42-video-two",
        }

        def download(url, timeout):
            self.assertEqual(120, timeout)
            self.assertIsInstance(url, str, "media download must not reuse an authenticated Request")
            self.assertNotIn("top-secret-key", url)
            return FakeDownload(media[url])

        temp = self._temp_dir()
        api_mock, sleep_mock, urlopen_mock = self.run_with_key(
            args(temp), lambda *call: next(api_responses), download
        )

        posts = [call for call in api_mock.call_args_list if len(call.args) > 3 and call.args[3] is not None]
        self.assertEqual(1, len(posts), "paid generation POST must happen exactly once")
        self.assertEqual("/v1/api/generate", posts[0].args[2])
        self.assertEqual(6, posts[0].args[3]["duration"])
        self.assertEqual("768p", posts[0].args[3]["resolution"])
        self.assertEqual("async", posts[0].args[3]["replyType"])
        self.assertEqual(3, len(api_mock.call_args_list) - len(posts))
        self.assertEqual(2, sleep_mock.call_count)
        self.assertEqual(2, urlopen_mock.call_count)
        self.assertEqual(media["https://media.example/one.mp4"], (temp / "video-0.mp4").read_bytes())
        self.assertEqual(media["https://media.example/two.mp4"], (temp / "video-1.mp4").read_bytes())
        summary = json.loads((temp / "summary.json").read_text(encoding="utf-8"))
        self.assertEqual("succeeded", summary["status"])
        self.assertEqual(2, summary["result_count"])
        self.assertEqual({"id": "task-123"}, json.loads((temp / "task.json").read_text(encoding="utf-8")))

    def test_task_id_resumes_without_post(self):
        temp = self._temp_dir()
        api_mock, _, urlopen_mock = self.run_with_key(
            args(temp, task_id="existing/task", timeout=10),
            [(200, {"status": "succeeded", "result": {"results": [
                {"url": "https://media.example/resumed.mp4"}
            ]}})],
            lambda *_args, **_kwargs: FakeDownload(b"\x00\x00\x00\x18ftypisom-resumed"),
        )

        self.assertEqual(1, api_mock.call_count)
        self.assertEqual(3, len(api_mock.call_args.args))
        self.assertEqual("/v1/api/result?id=existing%2Ftask", api_mock.call_args.args[2])
        self.assertFalse((temp / "task.json").exists())
        self.assertEqual(1, urlopen_mock.call_count)

    def test_polling_timeout_does_not_resubmit(self):
        temp = self._temp_dir()
        clock = iter([0.0, 0.0, 2.0])
        with mock.patch.dict(os.environ, {"SUB2API_KEY": "top-secret-key"}, clear=True), \
                mock.patch.object(video, "api", side_effect=[
                    (202, {"id": "task-timeout"}),
                    (200, {"status": "running"}),
                ]) as api_mock, \
                mock.patch.object(video.time, "monotonic", side_effect=lambda: next(clock)), \
                mock.patch.object(video.time, "sleep"), \
                mock.patch.object(video.urllib.request, "urlopen") as urlopen_mock:
            with self.assertRaisesRegex(RuntimeError, "resume with --task-id, do not resubmit"):
                video.run(args(temp, timeout=1))

        posts = [call for call in api_mock.call_args_list if len(call.args) > 3 and call.args[3] is not None]
        self.assertEqual(1, len(posts))
        self.assertEqual(2, api_mock.call_count)
        urlopen_mock.assert_not_called()

    def test_failed_and_manual_review_are_not_treated_as_success(self):
        for terminal in ("failed", "manual_review"):
            with self.subTest(status=terminal):
                temp = self._temp_dir()
                with self.assertRaisesRegex(RuntimeError, "Task terminal status: " + terminal):
                    self.run_with_key(
                        args(temp, task_id="terminal-task"),
                        [(200, {"status": terminal})],
                    )
                self.assertFalse((temp / "summary.json").exists())
                self.assertFalse((temp / "video-0.mp4").exists())

    def test_api_key_is_used_for_api_calls_but_never_media_download(self):
        temp = self._temp_dir()
        seen_api_keys = []
        seen_media = []

        def fake_api(base, key, path, body=None):
            seen_api_keys.append((key, path, body))
            return 200, {"status": "succeeded", "result": {"results": [
                {"url": "https://media.example/no-auth.mp4"}
            ]}}

        def fake_download(target, timeout):
            seen_media.append(target)
            return FakeDownload(b"\x00\x00\x00\x18ftypisom-safe")

        self.run_with_key(args(temp, task_id="safe-task"), fake_api, fake_download)
        self.assertEqual([("top-secret-key", "/v1/api/result?id=safe-task", None)], seen_api_keys)
        self.assertEqual(["https://media.example/no-auth.mp4"], seen_media)

    def _temp_dir(self):
        import tempfile
        path = Path(tempfile.mkdtemp(prefix="grsai-video-test-"))
        self.addCleanup(lambda: __import__("shutil").rmtree(path, ignore_errors=True))
        return path


if __name__ == "__main__":
    unittest.main()
