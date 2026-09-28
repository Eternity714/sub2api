# GRS.AI Async contract probe

The probe is offline by default and is not part of CI. It sends exactly one
`replyType=async` generation request only with `--live`, then polls `/result`.
It reports status and result count, never the task ID, image URLs, prompt,
response body, or API key. It does not test Sub2API billing or local S3 storage.

```powershell
python test/grsai_live_contract.py --self-test
python test/grsai_live_contract.py
```

For an authorized live check, set `GRSAI_BASE` (HTTPS international endpoint)
and `GRSAI_KEY` in the local process environment without putting credentials
in the repository or command history, then run:

```powershell
python test/grsai_live_contract.py --live --model <approved-model>
```

The live call may incur provider charges. The script limits itself to one
generation request and a 30-minute polling window; it does not enforce a
monetary spending cap. A successful provider probe must still be followed by
an authenticated Sub2API end-to-end check of local task IDs, S3 URLs, and
billing before release.

## GRS.AI 单请求多图探测

脚本直接调用上游原生 `POST /v1/api/generate`，一次请求带一个实验性数量字段，默认发送 `n=2`，随后使用返回的任务 ID 轮询 `/v1/api/result`。脚本只统计结果数量和 URL 去重数量，不下载图片，也不打印任务 ID、URL、提示词或响应体。

先运行无网络自测：

```powershell
python test/grsai_multi_image_probe.py --self-test
```

真实探测需要显式提供上游凭据，可能产生一次生成费用：

```powershell
$env:GRSAI_BASE = "https://grsaiapi.com"
$env:GRSAI_KEY = "[在当前进程中设置真实密钥]"
python test/grsai_multi_image_probe.py --live --model gpt-image-2 --count-field n --requested-count 2
```

如果上游不识别 `n`，可以在授权的测试额度下分别改用 `numImages`、`num_images`、`imageCount` 或 `image_count` 复测。每次运行只提交一个生成请求；脚本输出 `multiple_images_confirmed` 才表示同一请求返回了至少两张不同图片。`requested_count_not_returned` 表示请求成功但没有返回要求数量，`inconclusive*` 表示无法据此判断。
