"""GPT Image reference pages and OpenAPI derived from the public capability snapshot."""
import copy
import json
from pathlib import Path


def parameter_notes(model):
    name = model["model"]
    fixed = bool(model["schema"].get("fixed_image_resolution"))
    lines = ["## 参数使用说明", "",
             "### model 与 prompt", "",
             f'本页只接受 `model: "{name}"`。`prompt` 为必填字符串，长度 1–32000 个字符，描述主体、场景、构图、风格或参考图修改要求。', "",
             "### images：参考图", "",
             "文生图时省略 `images`。参考图生图时传入 1–16 个 HTTP(S) 图片直链，按数组顺序描述各图用途。没有参考图时不要传空数组 `[]` 或 `null`；不接受 Base64、data URL、本地路径和带用户名密码的 URL。链接应无需网页登录，并在任务处理期间有效。", "",
             "### aspectRatio：画面比例", "",
             "省略时为 `1:1`；`auto` 也按正方形构造请求，不会根据参考图自动继承比例。完整允许值见上方参数表。仅接受 `16:9` 这样的比例字符串，不接受 `1024x1024` 或 `2048x2048` 等像素尺寸。实际原图可能与下表目标尺寸存在差异。", "",
             "### imageSize：分辨率档位", ""]
    lines += (["本模型固定为 **1K**，请求中必须省略 `imageSize`、`resolution` 和 `metadata.resolution`，即使传 `1K` 也会返回 400。需要 2K/4K 或更多质量选项时，请选择[同版本 Flare](/media-models/" + name + "-flare)或[同版本 Sunburst](/media-models/" + name + "-sunburst)。", ""] if fixed else
              ["允许 `1K`、`2K`、`4K`，省略时为 `1K`，严格区分大小写。分辨率决定价格，`quality` 不改变本次计费档位。4K 的正方形目标为 **2880×2880**，不是 4096×4096；其他比例的目标尺寸见下表。", ""])
    lines += ["### quality：生成质量", ""]
    lines += (["仅接受 `auto`，也可省略。`low`、`medium`、`high`、`xhigh`、`max` 均未开放。每张统一标价 **0.08 元**。", ""] if fixed else
              ["允许 `auto`、`low`、`medium`、`high`，省略时为 `auto`（自动质量）；后三项分别请求低、中、高质量。同一 `imageSize` 下四种质量同价，改变质量不等于改变分辨率，也不承诺某个固定耗时或文件大小。`xhigh` 和 `max` 未开放。", ""])
    lines += ["### n 与 replyType", "",
              "`n` 默认为 `1`，且只能为 `1`。`replyType` 默认为 `async`，且只能为 `async`：先接收任务 ID，再轮询结果；不支持 `json` 同步返回或 `stream` 流式返回。", "",
              "### 未开放的参数", "",
              "`background`、`outputFormat`、`response_format`、`mask`、`webhook` 等未列出的字段不能传入。当前没有通过参数指定透明背景、GIF/SVG 输出或蒙版编辑的能力。字段名、字符串枚举、大小写或别名冲突不符时，创建前返回 400，不创建任务、不预留额度。", "",
              "六个型号的对照、Python / JavaScript 示例及 OpenAPI 下载见[GPT Image 系列调用指南](/gpt-image)。", ""]
    return lines


def response_notes(model):
    return ["### 处理中与失败", "",
            "处理中返回 `status:queued/in_progress`、`progress:0/50`，`result.data` 为空；此时继续 GET 查询。终态为 `completed/100`，必须同时检查 `result.data` 和 `error`。", "",
            "生成失败仍可能是 HTTP 200，例如：", "", "```json",
            json.dumps({"id": "media_EXAMPLE_TASK_ID", "model": model["model"], "status": "completed", "progress": 100, "created_at": 1794182400, "completed_at": 1794268800, "result": {"type": "image", "data": []}, "error": "media_task_expired"}, ensure_ascii=False, indent=2),
            "```", "",
            "上例没有图片，不应按成功处理；`expires_at` 仅在有成功结果时返回。请保存任务 ID，使用原 Key 继续查询；查询不会再次生成。不要将 `completed`、HTTP 200 或仅有 `id` 当作已生成图片。", "",
            "创建前的参数错误使用 HTTP 400，`error` 为包含 `type`、`message` 和可选 `fields` 的对象；任务查询中的 `error` 为字符串。完整错误码和客户端重试规则见[错误处理](/media-api#错误处理)。", ""]


def series_page(models, date):
    models = sorted(models, key=lambda m: m["model"])
    rows = []
    for model in models:
        fixed = bool(model["schema"].get("fixed_image_resolution"))
        pricing = model["pricing"]
        price = f'{pricing["unit_price"]:.2f} 元/张' if fixed else " / ".join(f'{p["per_request_price"]:.2f}' for p in pricing["intervals"]) + " 元/张"
        rows.append(f'| [{model["model"]}](/media-models/{model["model"]}) | ' + ("固定 1K，不传 `imageSize` | `auto` | 14" if fixed else "`1K` / `2K` / `4K` | `auto` / `low` / `medium` / `high` | 16") + f' | {price} |')
    base = next(m for m in models if m["model"] == "gpt-image-2.6-flare")
    specs = {(s["aspect_ratio"], s["resolution"]): f'{s["width"]}×{s["height"]}' for s in base["schema"]["image_specs"]}
    ratios = base["schema"]["fields"]["aspectRatio"]["enum"]
    sizes = ["| `" + ratio + "` | " + " | ".join(specs[("1:1" if ratio == "auto" else ratio, tier)] for tier in ["1K", "2K", "4K"]) + " |" for ratio in ratios]
    template = (Path(__file__).parent / "templates/gpt-image.md").read_text(encoding="utf-8")
    return template.replace("@@DATE@@", str(date)).replace("@@MODEL_TABLE@@", "\n".join(rows)).replace("@@SIZE_TABLE@@", "\n".join(sizes))


def _field_schema(field):
    value = {"type": field["type"]}
    for source, target in [("enum", "enum"), ("default", "default"), ("minimum", "minimum"), ("maximum", "maximum"), ("min_length", "minLength"), ("max_length", "maxLength"), ("min_items", "minItems"), ("max_items", "maxItems")]:
        if source in field:
            value[target] = field[source]
    if field.get("items"):
        value["items"] = _field_schema(field["items"])
    if field.get("format") == "http_url":
        value.update(format="uri", pattern="^https?://", description="无需登录的绝对 HTTP(S) URL；不能含用户名或密码，不支持 Base64 或 data URL。")
    return value


def openapi(models, date):
    schemas, mapping, examples = {}, {}, {}
    descriptions = {
        "model": "公开模型 ID，区分大小写。", "prompt": "提示词，描述图像内容或对参考图的修改要求。",
        "images": "可选参考图，1–16 个 HTTP(S) 图片直链；没有参考图时省略，不传空数组。",
        "aspectRatio": "画面比例；auto 也按1:1构造请求。仅允许比例，不接受像素尺寸写法。",
        "imageSize": "分辨率档位，默认1K，按此档位收费；4K正方形目标为2880×2880。",
        "quality": "质量选项，默认auto；同一分辨率下各允许质量同价。",
        "n": "每次仅生成一张。", "replyType": "仅async：返回任务ID，再轮询查询。",
    }
    for model in models:
        name = model["model"]
        key = "GPTImage" + name.removeprefix("gpt-image-").replace(".", "").replace("-", "_")
        props = {"model": {"type": "string", "enum": [name], "description": descriptions["model"]}}
        for field, definition in model["schema"]["fields"].items():
            props[field] = _field_schema(definition) | {"description": descriptions[field]}
        # Legacy aliases stay bounded by each model's public field rules.
        for canonical, alias in [("aspectRatio", "size"), ("images", "image_urls"), ("imageSize", "resolution")]:
            if canonical in props:
                props[alias] = copy.deepcopy(props[canonical])
                props[alias].pop("default", None)
                props[alias].update(deprecated=True, description=f"兼容字段，推荐使用{canonical}；与推荐字段同时传入时值必须一致。")
        metadata = {"quality": copy.deepcopy(props["quality"])}
        if "imageSize" in props:
            metadata["resolution"] = copy.deepcopy(props["imageSize"])
        for prop in metadata.values():
            prop.pop("default", None)
        props["metadata"] = {"type": "object", "additionalProperties": False, "deprecated": True, "properties": metadata, "description": "旧客户端的质量/分辨率兼容字段；与顶层字段同时传入必须一致。固定1K型号不能传metadata.resolution。"}
        schemas[key] = {"type": "object", "additionalProperties": False, "required": ["model", "prompt"], "properties": props, "description": name + ("：固定1K、quality仅auto，0.08元/张；禁止imageSize及分辨率别名。" if model["schema"].get("fixed_image_resolution") else "：1K/2K/4K分别0.12/0.20/0.35元/张；按分辨率收费。")}
        mapping[name] = "#/components/schemas/" + key
        for index, example in enumerate(model["examples"]):
            examples[key + "_" + str(index + 1)] = {"summary": name + " " + example["label"], "value": example["request"]}
    error = {"type": "object", "required": ["error"], "properties": {"error": {"type": "object", "required": ["type", "message"], "properties": {"type": {"type": "string"}, "message": {"type": "string"}, "fields": {"type": "array", "items": {"type": "object", "required": ["path", "code", "message"], "properties": {key: {"type": "string"} for key in ["path", "code", "message"]}}}}}}}
    created = {"type": "object", "required": ["id", "model", "status", "progress", "created_at"], "properties": {"id": {"type": "string", "pattern": "^media_"}, "model": {"type": "string"}, "status": {"type": "string", "enum": ["queued"]}, "progress": {"type": "integer", "enum": [0]}, "created_at": {"type": "integer", "format": "int64", "description": "Unix秒"}}}
    task = copy.deepcopy(created)
    task["required"] += ["result", "error"]
    task["properties"].update(status={"type": "string", "enum": ["queued", "in_progress", "completed"]}, progress={"type": "integer", "enum": [0, 50, 100]}, completed_at={"type": "integer", "format": "int64", "description": "终态返回，Unix秒。"}, expires_at={"type": "integer", "format": "int64", "description": "有成功结果时返回，Unix秒；过期返回410。"}, result={"type": "object", "required": ["type", "data"], "properties": {"type": {"type": "string", "enum": ["image"]}, "data": {"type": "array", "items": {"type": "string", "format": "uri"}, "description": "图片URL数组。处理中或失败时为空，成功交付一张。"}}}, error={"type": "string", "description": "无错误时为空字符串；completed仍须检查本字段与result.data。"})
    schemas.update(MediaImageCreated=created, MediaImageTask=task, MediaHTTPError=error)
    def response(description, schema):
        return {"description": description, "content": {"application/json": {"schema": {"$ref": "#/components/schemas/" + schema}}}}
    errors = {code: response(text, "MediaHTTPError") for code, text in {"400": "参数、模型或余额不符，不创建任务。", "401": "密钥无效。", "403": "无权限。", "404": "路径、模型或原密钥/任务不匹配。", "429": "额度或速率限制。", "500": "服务异常。", "503": "服务暂不可用。"}.items()}
    return {"openapi": "3.0.3", "info": {"title": "Gkotta GPT Image 生图 API", "version": str(date), "description": "六款GPT Image公开模型的创建与查询。只支持async、n=1；本文件为推荐参数及有界兼容别名。字段冲突、HTTP(S)链接及枚举仍由服务端校验。创建仅POST一次；保存id后使用原Key查询，不支持Idempotency-Key去重。"}, "servers": [{"url": "https://www.gkotta.bid"}], "security": [{"BearerAuth": []}], "tags": [{"name": "GPT Image", "description": "文生图与HTTP(S)参考图生图。"}], "paths": {
        "/v1/images/generations": {"post": {"tags": ["GPT Image"], "summary": "创建GPT Image生图任务", "operationId": "createGPTImage", "description": "Content-Type: application/json。请求体上限1MiB。HTTP202表示已受理，尚未交付图片；不支持同步或流式返回。", "requestBody": {"required": True, "content": {"application/json": {"schema": {"oneOf": [{"$ref": ref} for ref in mapping.values()], "discriminator": {"propertyName": "model", "mapping": mapping}}, "examples": examples}}}, "responses": {"202": response("已受理，请保存id并轮询。", "MediaImageCreated"), **errors}}},
        "/v1/images/generations/{task_id}": {"get": {"tags": ["GPT Image"], "summary": "查询生图状态与结果", "operationId": "getGPTImageTask", "description": "使用创建时的同一个API Key，每5–10秒查询。completed表示处理结束，成功需result.data有URL且error为空。", "parameters": [{"name": "task_id", "in": "path", "required": True, "description": "POST返回的本地media_任务ID。", "schema": {"type": "string", "pattern": "^media_", "maxLength": 100}}], "responses": {"200": response("任务状态；生成失败也可能是HTTP200。", "MediaImageTask"), "410": response("结果已过期。", "MediaHTTPError"), **{k: v for k, v in errors.items() if k != "403"}}}},
    }, "components": {"securitySchemes": {"BearerAuth": {"type": "http", "scheme": "bearer", "bearerFormat": "API Key", "description": "媒体API分组的Sub2API密钥。"}}, "schemas": schemas}}
