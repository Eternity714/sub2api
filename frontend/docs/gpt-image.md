---
description: 六款 GPT Image 生图模型的接口、参数、分辨率、质量、计费及完整异步调用示例。
---

# GPT Image 系列接口

核对日期：**2026-11-10**。六个公开型号共用创建和查询接口，支持文生图与参考图生图。先选模型，再按该模型的参数范围发送请求。

## 模型、参数与售价

| model（点击查看完整参数） | imageSize | quality | aspectRatio 值数（含 auto） | 标价 |
| --- | --- | --- | ---: | --- |
| [gpt-image-2.5](/media-models/gpt-image-2.5) | 固定 1K，不传 `imageSize` | `auto` | 14 | 0.08 元/张 |
| [gpt-image-2.5-flare](/media-models/gpt-image-2.5-flare) | `1K` / `2K` / `4K` | `auto` / `low` / `medium` / `high` | 16 | 0.12 / 0.20 / 0.35 元/张 |
| [gpt-image-2.5-sunburst](/media-models/gpt-image-2.5-sunburst) | `1K` / `2K` / `4K` | `auto` / `low` / `medium` / `high` | 16 | 0.12 / 0.20 / 0.35 元/张 |
| [gpt-image-2.6](/media-models/gpt-image-2.6) | 固定 1K，不传 `imageSize` | `auto` | 14 | 0.08 元/张 |
| [gpt-image-2.6-flare](/media-models/gpt-image-2.6-flare) | `1K` / `2K` / `4K` | `auto` / `low` / `medium` / `high` | 16 | 0.12 / 0.20 / 0.35 元/张 |
| [gpt-image-2.6-sunburst](/media-models/gpt-image-2.6-sunburst) | `1K` / `2K` / `4K` | `auto` / `low` / `medium` / `high` | 16 | 0.12 / 0.20 / 0.35 元/张 |

Flare / Sunburst 一行中的三个价格依次对应 **1K / 2K / 4K**。`gpt-image-2.5`、`gpt-image-2.6` 是固定 1K 简化型号：每张 **0.08 元**，不能传入 `imageSize`，质量仅 `auto`。同一型号与分辨率下，所有允许的质量同价。实际扣费价格及分组倍率以[可用渠道](https://www.gkotta.bid/available-channels)为准。

## 地址与认证

根地址：`https://www.gkotta.bid`。使用[媒体 API 分组的 API Key](https://www.gkotta.bid/keys)。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/v1/models` | 查询当前 Key 可见的公开模型 |
| POST | `/v1/images/generations` | 创建任务，成功为 HTTP **202** |
| GET | `/v1/images/generations/{task_id}` | 查询状态与图片 URL，成功为 HTTP **200** |

创建请求的 Header：

```http
Authorization: Bearer sk-YOUR_API_KEY
Content-Type: application/json
```

查询只需 `Authorization`。创建请求体上限 **1 MiB**。不需要额外填写平台、账号或内部型号。

## Body 参数

请求体为 JSON 对象。字符串枚举区分大小写；省略可选字段会填入该模型的默认值，传 `null` 不等于省略。

| 字段 | 类型 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | 无 | 上表六个公开模型之一；完整填写型号 |
| `prompt` | string | 是 | 无 | 提示词，1–32000 个字符；参考图场景可描述各图用途与修改要求 |
| `images` | string[] | 否 | 不传 | 参考图 HTTP(S) URL 数组，1–16 项；文生图省略，不传 `[]` |
| `aspectRatio` | string | 否 | `1:1` | 比例；`auto` 也按正方形构造请求；范围见下文 |
| `imageSize` | string | 按型号 | `1K`（仅 Flare/Sunburst） | 四款 Flare/Sunburst 允许 `1K/2K/4K`；两个简化型号禁止传入 |
| `quality` | string | 否 | `auto` | 简化型号仅 `auto`；Flare/Sunburst 为 `auto/low/medium/high` |
| `n` | integer | 否 | `1` | 每次仅允许一张，不能传 `2` 或字符串 `"1"` |
| `replyType` | string | 否 | `async` | 仅异步任务轮询；不接受 `json/stream` |

### aspectRatio

两个固定 1K 简化型号仅支持以下 **14** 个值：

```text
auto、1:1、16:9、9:16、4:3、3:4、3:2、2:3、5:4、4:5、21:9、9:21、1:2、2:1
```

四款 Flare/Sunburst 在 **1K、2K、4K** 三档均支持上述值，另外支持 `1:3`、`3:1`，共 **16** 个值。`auto` 和省略参数均按 `1:1` 构造请求，不继承参考图比例。禁止 `1024x1024` 等像素写法。

### imageSize 与目标像素

下表是请求的目标尺寸映射。**交付保留生成服务原图，实际像素允许存在差异**；不因差异自动缩放、裁剪、重试或改变计费档位。4K 正方形目标为 **2880×2880**，不能按 4096×4096 理解。固定 1K 简化型号只使用 1K 列，且不支持表中的 `1:3/3:1`。

| aspectRatio | 1K 目标像素 | 2K 目标像素 | 4K 目标像素 |
| --- | --- | --- | --- |
| `auto` | 1024×1024 | 2048×2048 | 2880×2880 |
| `1:1` | 1024×1024 | 2048×2048 | 2880×2880 |
| `16:9` | 1280×720 | 2560×1440 | 3840×2160 |
| `9:16` | 720×1280 | 1440×2560 | 2160×3840 |
| `4:3` | 1152×864 | 2304×1728 | 3264×2448 |
| `3:4` | 864×1152 | 1728×2304 | 2448×3264 |
| `3:2` | 1248×832 | 2496×1664 | 3504×2336 |
| `2:3` | 832×1248 | 1664×2496 | 2336×3504 |
| `5:4` | 1120×896 | 2240×1792 | 3200×2560 |
| `4:5` | 896×1120 | 1792×2240 | 2560×3200 |
| `21:9` | 1456×624 | 3024×1296 | 3808×1632 |
| `9:21` | 624×1456 | 1296×3024 | 1632×3808 |
| `1:2` | 720×1440 | 1440×2880 | 1920×3840 |
| `2:1` | 1440×720 | 2880×1440 | 3840×1920 |
| `1:3` | 576×1728 | 1168×3504 | 1280×3840 |
| `3:1` | 1728×576 | 3504×1168 | 3840×1280 |

### quality

`auto` 表示自动质量；`low/medium/high` 分别请求低、中、高质量。`quality` 不改变所选分辨率或本次标价，也不保证固定耗时、文件大小或可复现的画面。`xhigh/max` 暂未开放，不能用于任何一个下游型号。

### images 与未开放参数

参考图需是生成服务可直接下载的 HTTP(S) 图片文件。带签名的链接应在任务处理期间持续有效；不能依赖网页登录。没有参考图时省略 `images`，不要照其他平台示例传空数组。

当前不接受 Base64、data URL、本地文件路径、带用户名密码的 URL、`background`、`outputFormat`、`response_format`、`mask`、`webhook` 等参数。没有通过参数指定透明背景、GIF/SVG 输出、蒙版或同步/流式返回的能力。完整字段约束以各型号参数页为准；不要将其他平台文档中的可选字段直接加入请求。

### 旧参数兼容

新接入使用上方推荐字段。已有客户端可以在对应字段开放时使用 `size` → `aspectRatio`、`image_urls` → `images`、`metadata.quality` → `quality`；Flare/Sunburst 还兼容 `resolution` 或 `metadata.resolution` → `imageSize`。新旧字段同时传入必须一致。固定 1K 简化型号禁止所有分辨率字段，包括值为 `1K` 的兼容字段。

## 请求示例

### 最小文生图：固定 1K、0.08 元/张

```bash
export SUB2API_BASE_URL="https://www.gkotta.bid"
export SUB2API_KEY="sk-YOUR_API_KEY"

curl --fail-with-body "$SUB2API_BASE_URL/v1/images/generations" \
  -H "Authorization: Bearer $SUB2API_KEY" \
  -H 'Content-Type: application/json' \
  --data '{"model":"gpt-image-2.6","prompt":"白色桌面上的蓝色陶瓷杯，柔和自然光"}'
```

本例省略了 `images/aspectRatio/quality/n/replyType`，默认按 `1:1`、自动质量、1 张、异步处理，分辨率固定 1K。改成 `gpt-image-2.5` 使用同样参数。**不要给这两个型号添加 `imageSize`。**

### 参考图生图：Flare 2K、高质量

下面是完整 JSON 请求。使用上方相同的 Header 与 POST 地址，将参考图占位 URL 替换成自己的可下载图片直链。

```json
{
  "model": "gpt-image-2.6-flare",
  "prompt": "保留第一张参考图的主体和构图，使用第二张图的配色，将陶瓷杯改为绿色",
  "images": [
    "https://example.com/REPLACE_WITH_REFERENCE_1.png",
    "https://example.com/REPLACE_WITH_REFERENCE_2.png"
  ],
  "aspectRatio": "16:9",
  "imageSize": "2K",
  "quality": "high",
  "n": 1,
  "replyType": "async"
}
```

本例按 **2K、0.20 元/张**计费，`high` 不另外加价。四款 Flare/Sunburst 可以替换 `model` 后使用同样的参数规则；各型号的 1K/2K/4K 文生图及参考图示例见上方型号链接。

## 返回响应与结果查询

创建成功返回 **HTTP 202**，只有任务信息：

```json
{
  "id": "media_EXAMPLE_TASK_ID",
  "model": "gpt-image-2.6",
  "status": "queued",
  "progress": 0,
  "created_at": 1794182400
}
```

保存这个 `id`，每 **5–10 秒**使用**创建时的同一个 Key**查询。当前不支持 `Idempotency-Key` 去重，重复 POST 会创建新任务并分别计费；超时不代表未受理，拿到 ID 后只重试 GET。

```bash
export TASK_ID="media_EXAMPLE_TASK_ID" # 替换成真实创建响应中的 id
curl --fail-with-body "$SUB2API_BASE_URL/v1/images/generations/$TASK_ID" \
  -H "Authorization: Bearer $SUB2API_KEY"
```

| status | progress | 处理方式 |
| --- | ---: | --- |
| `queued` | 0 | 等待后继续查询 |
| `in_progress` | 50 | 继续查询，尚无图片交付 |
| `completed` | 100 | 停止轮询，检查 `result.data` 与 `error` |

成功结果（HTTP 200）：

```json
{
  "id": "media_EXAMPLE_TASK_ID",
  "model": "gpt-image-2.6",
  "status": "completed",
  "progress": 100,
  "created_at": 1794182400,
  "completed_at": 1794182460,
  "expires_at": 1794268860,
  "result": {
    "type": "image",
    "data": ["https://files.example.com/media/result.png"]
  },
  "error": ""
}
```

| 响应字段 | 类型与规则 |
| --- | --- |
| `id`、`model` | 本地任务 ID 与请求的公开模型；查询使用这个 ID |
| `status`、`progress` | 上表所列状态与固定进度 |
| `created_at` | Unix 秒，创建时间 |
| `completed_at` | Unix 秒，仅终态返回 |
| `expires_at` | Unix 秒，仅有成功结果时返回，过期后查询为 410 |
| `result.type` | `image` |
| `result.data` | 图片 URL 字符串数组；处理中或失败时为空 |
| `error` | 字符串，成功时为空；失败时为错误摘要 |

`completed` 表示处理结束，**不是成功承诺**。只有 `result.data` 有图片 URL 且 `error` 为空才是完整成功；`completed`、空数组、非空 `error` 表示失败，HTTP 仍可能是 200，例如：

```json
{
  "id": "media_EXAMPLE_TASK_ID",
  "model": "gpt-image-2.6",
  "status": "completed",
  "progress": 100,
  "created_at": 1794182400,
  "completed_at": 1794268800,
  "result": {"type":"image","data":[]},
  "error": "media_task_expired"
}
```

用 `result.data[0]` 中的完整 URL 下载文件，在 `expires_at` 前保存。**不要向文件 URL 附加 API Key**，不要依赖 URL 后缀判断实际文件格式。

## Python：创建一次并轮询

示例使用 Python 3 标准库。先设置 `SUB2API_KEY`；可选设置 `GPT_IMAGE_MODEL` 为上表任一型号。`MEDIA_TASK_ID` 可恢复查询已有任务，避免重复创建。

```python
import json, os, time, urllib.error, urllib.request

base = "https://www.gkotta.bid"
headers = {"Authorization": "Bearer " + os.environ["SUB2API_KEY"],
           "Content-Type": "application/json"}
model = os.getenv("GPT_IMAGE_MODEL", "gpt-image-2.6")
body = {"model": model, "prompt": "白色桌面上的蓝色陶瓷杯，柔和自然光",
        "aspectRatio": "1:1", "quality": "auto", "n": 1, "replyType": "async"}
if model.endswith(("-flare", "-sunburst")):
    body["imageSize"] = "2K"  # 只给这四款型号传入

task_id = os.getenv("MEDIA_TASK_ID")
if not task_id:
    request = urllib.request.Request(base + "/v1/images/generations",
        data=json.dumps(body, ensure_ascii=False).encode("utf-8"), headers=headers)
    # POST只发送一次；网络异常/超时后不要自动重新创建。
    with urllib.request.urlopen(request, timeout=60) as response:
        if response.status != 202:
            raise RuntimeError("创建响应不是HTTP 202")
        task_id = json.load(response)["id"]
    print("请保存任务ID：", task_id, flush=True)

deadline = time.monotonic() + 30 * 60  # 客户端等待上限，不代表服务端任务失败
while time.monotonic() < deadline:
    try:
        request = urllib.request.Request(base + "/v1/images/generations/" + task_id,
                                         headers=headers)
        with urllib.request.urlopen(request, timeout=30) as response:
            task = json.load(response)
    except urllib.error.HTTPError as error:
        if error.code != 429 and error.code < 500:
            raise  # 鉴权、404或410等需要处理，不盲目继续
        time.sleep(10)
        continue
    except (urllib.error.URLError, TimeoutError):
        time.sleep(5)  # 只重试GET，不重新POST
        continue
    if task["status"] == "completed":
        if task.get("error") or not task["result"]["data"]:
            raise RuntimeError(task.get("error") or "没有图片结果")
        print("请在过期前下载：", task["result"]["data"][0])
        break
    if task["status"] not in ("queued", "in_progress"):
        raise RuntimeError("未知任务状态")
    time.sleep(5)
else:
    raise TimeoutError("客户端等待超时，请保留任务ID稍后继续GET查询：" + task_id)
```

JavaScript / Node.js 的完整创建与轮询实现见[媒体 API 示例](/media-api#node-js-轮询示例)；将模型替换为上表型号，固定 1K 简化型号不要加 `imageSize`。Windows 示例见[PowerShell 调用](/media-images#windows-powershell)。

## 错误处理与计费

创建前参数错误返回 HTTP 400，`error` 为对象，可能包含 `fields` 字段详情；模型页中的未声明字段、错误枚举、空参考图数组以及固定型号的分辨率参数都会在创建前被拒绝，不创建任务、不预留额度。查询响应的 `error` 则为字符串。

| HTTP | 含义与处理 |
| --- | --- |
| 400 | 检查模型、参数、别名冲突或可用余额，修正后再提交 |
| 401 / 403 | 检查 Key、分组和权限 |
| 404 | 检查路径、真实任务 ID 及原始 Key；同一用户的另一个 Key 不能查询 |
| 410 | 结果已过期，使用之前下载的文件 |
| 429 / 5xx | 查询可延后重试；POST 未知受理状态时不要盲目重复生成 |
| 200 且 completed + error | 已创建任务的生成失败，读取错误字符串处理 |

成功创建时预留额度，图片生成并完成保存后结算；没有成功交付图片时释放预留。按创建时的公开模型与分辨率价格收费，不因内部服务切换、质量档位或结果像素偏差重复加价。每次请求最多交付一张。

## OpenAPI 文件

[下载 GPT Image OpenAPI 3.0 JSON](/openapi/gpt-image.json)，可按 URL 或文件导入 Apifox。包含六款模型各自的请求 Schema、文生图与参考图示例、Bearer 认证、HTTP 202 创建响应、任务查询结构和错误响应。按 `model` 区分 Schema；服务端仍会检查别名冲突和 HTTP(S) 链接规则。

[当前全部模型](/media-models/) · [图片生成 API](/media-images) · [统一协议、结果下载与错误码](/media-api)
