#!/usr/bin/env python3
"""Generate public model documentation from the reviewed capability snapshot."""
import argparse
import hashlib
import json
from datetime import datetime
from pathlib import Path
from zoneinfo import ZoneInfo

ROOT = Path(__file__).resolve().parents[1]
SOURCE = ROOT / "docs/media-api-catalog.json"
TICK = chr(96)
FENCE = TICK * 3
DESCRIPTIONS = {
    "model": "模型 ID，区分大小写，必须与本页完全一致。",
    "prompt": "描述要生成的内容或对参考图的修改要求。",
    "size": "图片宽高比，格式为正整数 a:b，不是像素尺寸。",
    "n": "请求生成的图片结果数量。",
    "duration": "视频时长，单位为秒。",
    "aspect_ratio": "视频宽高比，部分参考图模式禁止显式传入。",
    "image_urls": "参考图 URL 列表。传入前请确认素材可公开读取。",
    "metadata": "模型扩展参数对象，仅接受下列已声明的字段。",
    "resolution": "输出分辨率档位，严格使用所列字符串及大小写。",
    "seed": "随机种子，用于控制生成的随机性；不保证相同种子每次生成完全相同的文件。",
    "watermark": "是否启用模型水印。",
    "aigc_watermark": "是否添加 AI 生成内容水印。",
    "audio": "是否生成音频，仅列出的值可用。",
    "audio_type": "音频内容：all 为全部，speech_only 为语音，sound-effect_only 为音效。",
    "camerafixed": "是否固定摄像机，减少镜头移动。",
    "background": "背景选项，transparent 表示透明背景。",
    "quality": "生成质量档位，仅列出的值可用。",
    "response_format": "结果图片文件格式，不改变媒体 API 的 URL 数组响应结构。",
    "safety_tolerance": "内容安全容忍度设置，允许范围见约束。",
    "prompt_upsampling": "是否自动扩展提示词以补充细节。",
    "prompt_extend": "是否扩展提示词。",
    "prompt_optimizer": "是否优化提示词。",
    "optimize_prompt_options": "提示词优化选项对象。",
    "metadata.optimize_prompt_options.mode": "提示词优化方式：standard 为标准，fast 为快速。",
    "orientation": "画面方向：landscape 为横向，portrait 为纵向。",
    "grounding": "检索辅助：none 为关闭，web 为网页，image 为图片，web_image 为两者。",
    "google_search": "是否启用网页搜索辅助。",
    "google_image_search": "是否启用图片搜索辅助，组合条件见输入模式。",
    "topP": "采样概率阈值，用于控制输出多样性。",
    "temperature": "采样温度，用于控制输出随机性。",
    "maxOutputTokens": "输出 Token 上限；这是模型参数，计费方式仍见本页计费说明。",
    "personGeneration": "人物生成策略：ALLOW_ALL 允许人物，ALLOW_ADULT/allow_adult 仅成人，ALLOW_NONE/dont_allow 不生成人物。",
    "imageOutputOptions": "图片文件输出选项对象。",
    "mimeType": "输出图片的 MIME 类型。",
    "compressionQuality": "文件压缩质量选项，字符串或数值范围以本字段类型为准。",
    "safetySettings": "内容安全设置列表。",
    "category": "内容安全类别：仇恨、危险、色情或骚扰内容。",
    "threshold": "安全拦截阈值：OFF 关闭，BLOCK_LOW_AND_ABOVE 低风险及以上，BLOCK_MEDIUM_AND_ABOVE 中风险及以上，BLOCK_ONLY_HIGH 仅高风险。",
    "generation_type": "参考素材生成方式，frame 表示帧输入。",
    "video_generation_mode": "视频生成模式，reference_images_to_video 表示参考图生成视频。",
    "task": "输入任务类型：text_to_video 为文字，image_to_video 为图片，reference_to_video 为参考素材生成视频。",
    "action": "生成方式：text-to-video 为文生视频，image-to-video 为图生视频。",
    "mode": "视频生成质量模式，std 为标准模式。",
    "negative_prompt": "希望画面避免出现的内容。",
    "negativePrompt": "希望画面避免出现的内容。",
    "generateAudio": "是否生成音频，仅列出的值可用。",
    "resizeMode": "参考图适配方式：pad 为填充，crop 为裁剪。",
    "enable_gif": "是否生成 GIF；若仅列 false，表示此功能未开放。",
    "fast_pretreatment": "是否启用快速素材预处理。",
    "shot_type": "镜头类型：single 为单镜头，multi 为多镜头。",
}
ORDER = ["prompt", "size", "n", "duration", "aspect_ratio", "image_urls", "metadata"]


def dump(value):
    return json.dumps(value, ensure_ascii=False, separators=(",", ":"))


def code(value):
    text = value if isinstance(value, str) else dump(value)
    return TICK + text.replace("|", "&#124;") + TICK


def literal(value):
    return code(dump(value))


def paths(values):
    return "、".join(map(code, values))


def description(path):
    path = path.replace("[]", "")
    return DESCRIPTIONS.get(path) or DESCRIPTIONS[path.split(".")[-1]]


def constraints(field):
    result = []
    if "enum" in field:
        result.append("仅限 " + "、".join(map(literal, field["enum"])))
    for key, label in [("minimum", "最小值"), ("maximum", "最大值"), ("min_length", "最少字符"), ("max_length", "最多字符"), ("min_items", "最少元素"), ("max_items", "最多元素")]:
        if key in field:
            result.append(f'{label} {field[key]}')
    if field.get("format") == "http_url":
        result.append("无用户名或密码的绝对 HTTP(S) URL")
    result.append("允许 null" if field.get("nullable") else "不接受 null")
    return "；".join(result)


def field_rows(fields, prefix=""):
    result = []
    for key in sorted(fields, key=lambda key: (ORDER.index(key) if key in ORDER else 100, key)):
        field = fields[key]
        path = f"{prefix}.{key}" if prefix else key
        required = ("父对象存在时必填" if prefix else "必填") if field.get("required") else "可选"
        if "default" in field:
            required += "，可省略并采用默认"
        default = literal(field["default"]) if "default" in field else "未声明"
        result.append(f'| {code(path)} | {code(field["type"])} | {required} | {default} | {description(path)} | {constraints(field)} |')
        if "properties" in field:
            result.extend(field_rows(field["properties"], path))
        if "items" in field:
            item = field["items"]
            desc = "单个配置对象，字段如下。" if item["type"] == "object" else "单个参考图片 URL。"
            result.append(f'| {code(path + "[]")} | {code(item["type"])} | 每个数组元素 | 未声明 | {desc} | {constraints(item)} |')
            if "properties" in item:
                result.extend(field_rows(item["properties"], path + "[]"))
    return result


def condition(cond):
    field = code(cond["path"])
    return {
        "present": lambda: f"传入 {field}",
        "absent": lambda: f"不传 {field}",
        "empty": lambda: f"{field} 未传或为空",
        "not_empty": lambda: f"{field} 已传且非空",
        "equals": lambda: f'{field} 等于 {literal(cond.get("value"))}',
    }[cond["op"]]()


def rule_text(rule):
    values = rule["paths"]
    result = {
        "at_least_one": lambda: f"至少传入以下一个字段：{paths(values)}。",
        "mutually_exclusive": lambda: f"以下字段互斥，最多传入一个：{paths(values)}。",
        "require_together": lambda: f"以下字段必须一起传入或全部省略：{paths(values)}。",
        "requires": lambda: f"传入 {code(values[0])} 时，必须同时传入 {paths(values[1:])}。",
    }[rule["kind"]]()
    if rule.get("when"):
        result = "当" + "，且".join(map(condition, rule["when"])) + "时：" + result
    return result


def slug(model, models):
    if model["model"] == "MiniMax-H3":
        return "minimax-h3-uppercase"
    value = model["model"].lower()
    if sum(other["model"] == model["model"] for other in models) > 1:
        value += "-" + model["media_type"]
    return value


def fenced(value, language="json"):
    return [FENCE + language, json.dumps(value, ensure_ascii=False, indent=2), FENCE, ""]


def model_page(model, date):
    name, schema, media = model["model"], model["schema"], model["media_type"]
    image = media == "image"
    kind = "图片" if image else "视频"
    endpoint = "/v1/" + ("images" if image else "videos") + "/generations"
    fields = schema["fields"]
    lines = ["---", f"description: {name} {kind}生成 API 参数、输入模式、调用示例与结果查询。", "---", "", f"# {name}", "",
             f"{kind}生成模型，使用[媒体 API 异步协议](/media-api)。参数核对日期：{date}。", "",
             f"**模型 ID：{code(name)}**。请原样填写，大小写不可替换。", ""]
    if name in ["MiniMax-H3", "minimax-h3"]:
        lines += ["::: warning 模型 ID 区分大小写", TICK + "MiniMax-H3" + TICK + " 与 " + TICK + "minimax-h3" + TICK + " 是两项独立能力，支持的时长、分辨率和参数不同。请按当前页调用。", ":::", ""]
    lines += ["## 接口与认证", "", "| 方法 | 路径 | 用途 |", "| --- | --- | --- |",
              f"| POST | {code(endpoint)} | 创建任务，成功返回 HTTP 202 |",
              f'| GET | {code(endpoint + "/{task_id}")} | 使用创建响应的 id 查询状态和结果 |', "",
              "根地址为 " + code("https://www.gkotta.bid") + "；请求头使用 " + code("Authorization: Bearer sk-YOUR_API_KEY") + "。API Key 必须关联已开放本模型的媒体 API 分组。可用性与实际价格以[可用渠道](https://www.gkotta.bid/available-channels)为准。", "",
              "## 请求参数", "", "以下为本模型当前开放的完整字段。顶层或嵌套对象中未列出的字段均不能传入；类型、枚举、范围或组合不符时返回 " + code("400") + "，不创建任务、不预留额度。", "",
              "| 字段 | 类型 | 必填性 | 默认值 | 用途 | 约束 |", "| --- | --- | --- | --- | --- | --- |",
              f'| {code("model")} | {code("string")} | 必填 | 无 | {DESCRIPTIONS["model"]} | 仅限 {literal(name)} |']
    lines += field_rows(fields) + ["", "表中必填性按默认值填充后的请求判断。模式附加的必填和禁止字段见下节；嵌套字段的必填性仅在父对象存在时判断。默认值不代表任意扩展参数都会自动补齐。", ""]
    meta = fields.get("metadata")
    if meta:
        lines += [(f'省略 {code("metadata")} 时，本模型先采用默认 {literal(meta["default"])}，再补入其已声明的子字段默认值。' if "default" in meta else "本模型未声明 " + code("metadata") + " 父对象默认值；省略整个对象时，不会应用其子字段的默认值。需要相关选项时请显式传入对象。"), ""]
    if "image_urls" in fields:
        lines += ["参考图必须替换为调用方实际可公开访问的 HTTP(S) 文件 URL；示例 " + code("example.com") + " 仅占位，不能直接用来生成。不支持在 " + code("image_urls") + " 中放入 Base64 字符串或本地文件路径。", ""]
    lines += ["## 输入模式与组合限制", ""]
    if schema.get("modes"):
        lines += ["补入默认值后，请求必须且只能匹配下表的一种模式。未传与 " + code("null") + " 不等价，仅当字段明确允许时才能传 " + code("null") + "。", "", "| 模式 | 同时满足的条件 | 附加必填 | 禁止传入 |", "| --- | --- | --- | --- |"]
        for mode in schema["modes"]:
            conditions = "；".join(map(condition, mode.get("when", []))) or "无附加条件"
            lines.append(f'| {code(mode["id"])} | {conditions} | {paths(mode.get("required", [])) or "无"} | {paths(mode.get("forbidden", [])) or "无"} |')
        lines += [""]
    else:
        lines += ["本模型无额外的模式匹配条件，按上表的字段约束校验。参考图是否必填、可传数量以 " + code("image_urls") + " 行为准。", ""]
    lines += ["- " + rule_text(rule) for rule in schema.get("rules", [])]
    if schema.get("rules"):
        lines.append("")
    if image:
        lines += [code("size") + " 仅表示宽高比，分辨率放在 " + code("metadata.resolution") + "（仅在上表声明时可传）；不能传 " + code("metadata.output_size") + "。", ""]
        if schema.get("image_specs"):
            lines += ["### 有效比例与分辨率组合", "", "只接受以下成对组合，不能把比例和分辨率枚举任意交叉。像素尺寸是该组合的配置映射。", "", "| size | metadata.resolution | 像素尺寸 |", "| --- | --- | --- |"]
            for spec in schema["image_specs"]:
                lines.append(f'| {code(spec["aspect_ratio"])} | {code(spec["resolution"])} | {spec["width"]} × {spec["height"]} |')
            lines.append("")
        else:
            lines += ["当前能力未声明精确像素映射，分辨率是档位说明；本页不承诺某个固定像素尺寸。", ""]
    else:
        lines += ["视频请求不接受顶层 " + code("size") + " 或 " + code("n") + "。分辨率字符串严格区分大小写，例如 " + code("720p") + " 与 " + code("720P") + " 不可自行替换。", ""]
    lines += ["## 创建任务示例", "", "以下 cURL 使用 Bash。设置当前密钥，示例不会自动提交或重试：", "", FENCE + "bash", 'export SUB2API_BASE_URL="https://www.gkotta.bid"', 'export SUB2API_KEY="sk-YOUR_API_KEY"', FENCE, ""]
    for example in model["examples"]:
        lines += ["### " + example["label"], ""]
        if example.get("mode"):
            lines += ["对应输入模式：" + code(example["mode"]) + "。", ""]
        request = json.dumps(example["request"], ensure_ascii=False, indent=2)
        if "'" in request:
            raise ValueError("Example needs shell escaping: " + name)
        lines += [FENCE + "bash", f'curl --fail-with-body "$SUB2API_BASE_URL{endpoint}" \\',
                  '  -H "Authorization: Bearer $SUB2API_KEY" \\', "  -H 'Content-Type: application/json' \\",
                  "  --data '" + request + "'", FENCE, ""]
    lines += ["成功响应（HTTP 202）：", ""] + fenced({"id": "media_EXAMPLE_TASK_ID", "model": name, "status": "queued", "progress": 0, "created_at": 1792108800})
    lines += ["## 查询状态与获取结果", "", "保留创建响应的 " + code("id") + "，使用创建时的同一个 API Key 查询，建议间隔 5–10 秒：", "",
              FENCE + "bash", 'export TASK_ID="media_EXAMPLE_TASK_ID"',
              f'curl --fail-with-body "$SUB2API_BASE_URL{endpoint}/$TASK_ID" \\',
              '  -H "Authorization: Bearer $SUB2API_KEY"', FENCE, "",
              code("queued/0") + " 为排队，" + code("in_progress/50") + " 为处理中，" + code("completed/100") + " 为已结束。只有 " + code("completed") + " 才读取最终结果；完成并不表示成功。", "", "成功结果示例（HTTP 200）：", ""]
    lines += fenced({"id": "media_EXAMPLE_TASK_ID", "model": name, "status": "completed", "progress": 100, "created_at": 1792108800, "completed_at": 1792109100, "expires_at": 1792195500, "result": {"type": media, "data": ["https://files.example.com/media/result." + ("png" if image else "mp4")]}, "error": ""})
    lines += [code("result.data") + " 为文件 URL 字符串数组；无成功结果时为空，有错误时由 " + code("error") + " 字符串说明。所有终态返回 " + code("completed_at") + "，仅有成功结果时返回 " + code("expires_at") + "，时间均为 Unix 秒。请在过期前下载自己的实际结果 URL。", "",
              "完整响应规则、部分成功、错误码与轮询程序见[媒体 API 接入指南](/media-api#查询任务与下载结果)。模型或媒体类型不匹配、字段非法、未开放能力会在创建前拒绝；创建成功后的执行失败通过查询响应的 " + code("completed + error") + " 返回。", "", "## 计费与交付", ""]
    units = {"image": "按成功交付的图片结果数计费", "video": "按成功交付的视频时长（秒）计费", "per_request": "按成功交付的生成次数计费"}
    lines += [units[model["pricing"]["billing_mode"]] + "。创建任务时预留额度，全部文件保存及任务处理完成后结算；失败部分释放预留额度。", ""]
    tiers = [tier["tier_label"] for tier in model["pricing"].get("intervals", []) if tier.get("tier_label")]
    if tiers:
        lines += ["当前价目档位：" + paths(tiers) + "。价目档位不扩展可用参数范围；请求仍须满足本页字段及模式限制。", ""]
    if image and "n" in fields:
        lines += ["批量图片等待整项任务结束后统一返回，部分成功时仅交付成功的文件并按成功部分收费。支持数量以 " + code("n") + " 的约束为准。", ""]
    lines += ["实际单价及分组倍率以[可用渠道](https://www.gkotta.bid/available-channels)显示为准。", "", "[返回媒体模型目录](/media-models/) · [统一协议与错误处理](/media-api)"]
    return "\n".join(lines) + "\n"


def generate(catalog):
    models = catalog["models"]
    date = datetime.fromisoformat(catalog["snapshot_at"]).astimezone(ZoneInfo("Asia/Shanghai")).date()
    outputs, slugs = {}, set()
    for model in models:
        assert model["callable"] and model["pricing_available"] and model["examples"], model["model"]
        value = slug(model, models)
        assert value.casefold() not in slugs, value
        slugs.add(value.casefold())
        outputs[f"frontend/docs/media-models/{value}.md"] = model_page(model, date)
    groups = [("image", "图片模型"), ("video", "视频模型")]
    group_models = lambda media: sorted((model for model in models if model["media_type"] == media), key=lambda model: (model["model"].lower(), model["model"]))
    lines = ["---", "description: 媒体 API 已开放模型目录，每个模型的参数、输入模式、输出规格和调用示例。", "---", "", "# 媒体模型目录", "",
             f'本目录包含 {len(models)} 个当前开放的模型能力（{len(group_models("image"))} 个图片、{len(group_models("video"))} 个视频），参数核对日期：{date}。点击模型查看完整参数和可复制示例。', "",
             "请先阅读[媒体 API 接入指南](/media-api)，确认 API Key 使用媒体 API 分组。模型 ID 区分大小写，可用模型与实际价格以[可用渠道](https://www.gkotta.bid/available-channels)为准。", "",
             "图片统一使用 " + code("POST /v1/images/generations") + "，视频统一使用 " + code("POST /v1/videos/generations") + "；随后通过对应的 " + code("GET .../generations/{task_id}") + " 查询结果。所有模型均为异步任务。", "",
             "每个模型只接受其页面列出的字段。不同模型的分辨率、时长、参考图及音频限制各不相同，不要直接复用其它模型的扩展参数。未开放型号不在本目录中列出。", ""]
    sidebar = []
    for media, label in groups:
        lines += ["## " + label, "", "| 模型 ID 与文档 | 分辨率档位 | 参考图 |", "| --- | --- | --- |"]
        items = []
        for model in group_models(media):
            link = "/media-models/" + slug(model, models)
            items.append({"text": model["model"], "link": link})
            fields = model["schema"]["fields"]
            resolutions = fields.get("metadata", {}).get("properties", {}).get("resolution", {}).get("enum")
            images = fields.get("image_urls")
            support = "未开放"
            if images:
                support = "支持"
                if images.get("required"):
                    support = "必填（允许空数组）" if not images.get("min_items") else "必填"
                if images.get("max_items"):
                    support += f'，最多 {images["max_items"]} 张'
                support += "，以模式限制为准"
            res = "、".join(map(code, resolutions)) if resolutions else "见模型参数"
            lines.append(f'| [{model["model"]}]({link}) | {res} | {support} |')
        lines.append("")
        sidebar.append({"text": f"{label}（{len(items)}）", "collapsed": True, "items": items})
    outputs["frontend/docs/media-models/index.md"] = "\n".join(lines) + "\n"
    outputs["frontend/docs/.vitepress/media-models.generated.ts"] = "// Generated by deploy/generate-media-api-docs.py. Do not edit.\nexport const mediaModelSidebar = " + json.dumps(sidebar, ensure_ascii=False, indent=2) + "\n"
    return outputs


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    source = SOURCE.read_bytes()
    catalog = json.loads(source)
    outputs = generate(catalog)
    import re
    assert not re.search(r"toapis|grs[.\s_-]*ai", source.decode("utf-8"), re.I), "Provider identifier in catalog"
    stale = []
    for path, content in outputs.items():
        assert not re.search(r"toapis|grs[.\s_-]*ai", content, re.I), path
        target = ROOT / path
        if args.check:
            if not target.is_file() or target.read_text(encoding="utf-8") != content:
                stale.append(path)
        else:
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_text(content, encoding="utf-8", newline="\n")
    for target in (ROOT / "frontend/docs/media-models").glob("*.md"):
        if target.relative_to(ROOT).as_posix() not in outputs:
            stale.append(f"Unexpected model page: {target}")
    if stale:
        raise SystemExit("Generated documentation is stale:\n" + "\n".join(stale))
    print(dump({"action": "checked" if args.check else "generated", "models": len(catalog["models"]), "pages": len(outputs) - 1, "snapshot": catalog["snapshot_at"], "catalog_sha256": hashlib.sha256(source).hexdigest()}))


if __name__ == "__main__":
    main()
