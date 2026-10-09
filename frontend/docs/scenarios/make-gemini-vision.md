# Make.com 接入 Gemini 图像理解

Make.com 的 HTTP 模块可以直接调用 Gkotta 的 Gemini 原生接口，将图片分析结果传给表格、数据库等后续模块。本文使用 `inlineData` 发送 Base64 图片，避免依赖模型能够直接读取任意外部图片网址。

## 准备图片和密钥

登录 [Make.com](https://www.make.com/)，在 [Gkotta API 密钥](https://www.gkotta.bid/keys)创建密钥。到[可用渠道](https://www.gkotta.bid/available-channels)确认分组支持 Gemini 原生协议，所选 `YOUR_MODEL_ID` 支持图片输入。

先准备一张较小的 PNG 或 JPEG 测试图片，例如包含两行英文的截图，并取得它的 Base64 字符串。可在自己的电脑上把 `test.png` 转换成 Base64：

```bash
python -c "import base64,pathlib; print(base64.b64encode(pathlib.Path('test.png').read_bytes()).decode())"
```

保存输出供第一次测试使用。JSON 中只需要 Base64 编码本体，不包括 `data:image/png;base64,` 前缀。图片格式与 `mimeType` 必须一致，实际大小限制以模型官方说明和当前分组能力为准。

## 创建 HTTP 请求模块

1. 新建 Scenario，添加 **HTTP → Make a request**。
2. Method 选择 **POST**，URL 填：

```text
https://www.gkotta.bid/v1beta/models/YOUR_MODEL_ID:generateContent
```

3. 替换模型 ID；不要包含重复的 `models/`，保留方法名前的冒号。
4. 新版 HTTP 模块可创建 **API key** 凭证，放置位置选择请求头，名称设置为 `x-goog-api-key`，值填写 `sk-YOUR_API_KEY`。如果使用旧版 Raw 模块，在 Headers 手动添加同名请求头。
5. Headers 增加 `Content-Type: application/json`。
6. Body 选择 JSON；旧版使用 **Raw → JSON (application/json)**。填入下面的请求体。

```json
{
  "contents": [
    {
      "role": "user",
      "parts": [
        {
          "text": "请描述这张图片，并提取其中的文字。用中文回答。"
        },
        {
          "inlineData": {
            "mimeType": "image/png",
            "data": "BASE64_IMAGE_DATA"
          }
        }
      ]
    }
  ]
}
```

把 `BASE64_IMAGE_DATA` 替换成上一步的实际字符串。使用 JPEG 时改为 `image/jpeg`。先保留最小请求，不加入工具、生成图片或其他尚未验证的参数。

## Run once 与响应提取

点击 **Run once**，展开 HTTP 模块输出，确认状态成功，且响应 `candidates` 中有内容。文本一般位于 API JSON 的 `candidates[0].content.parts` 中，可能分为多个 `text` 块。

HTTP 模块可开启解析响应；旧版若返回原始字符串，连接 **JSON → Parse JSON** 后再查看候选项。Make 的映射界面通常以第一个数组元素展示数据，和 JSON 中从 0 开始的下标不同，应直接从执行输出选择第一条 candidate 中的 `text`。

再接一个简单的文本/变量或表格模块，映射分析文字，执行一次，确认后续模块收到的是实际结果。到 [Gkotta 使用记录](https://www.gkotta.bid/usage)核对模型与消耗。

## 扩展为图片处理工作流

基础调用验证后，把上游邮件附件或文件模块提供的 Base64 字符串映射到 `inlineData.data`，同时映射实际 MIME 类型。使用 JSON 构建方式传递文本，避免把包含引号或换行的提示词直接拼进 Raw JSON。

如果上游只提供图片 URL，需要先下载图片并在自己的工作流中获得 Base64 数据。不要把需登录的网址当作图片数据，也不要默认任意 URL 都是 Gemini 的有效 `fileData.fileUri`；Files API 有独立的上传与 URI 规则，需另行确认支持。

批量图片使用迭代器逐张处理，并控制并发和重试次数。Scenario 运行记录可能包含图片及响应，按实际使用范围管理记录；密钥优先保存在 HTTP 凭证中，分享 Scenario 时移除凭证和真实测试图片。

## 常见问题

- **401/403**：检查 `x-goog-api-key` 凭证是否真正放到 Header，核对密钥、分组、有效期和限制。
- **404**：确认 `/v1beta/models/模型ID:generateContent` 路径，模型 ID 不包含重复 `models/`，协议不能写成 `/v1/chat/completions`。
- **400 或图片解析失败**：检查 Base64 是否完整、是否误加 data URL 前缀、MIME 是否匹配实际图片；先用更小图片复测。
- **响应成功但没有文本**：检查全部候选和内容块，以及安全过滤或错误字段，不要直接认定 `parts[0].text` 一定存在。
- **下游模块字段为空**：确认 HTTP 已解析 JSON，或经过 Parse JSON；从一次真实执行的输出重新选择映射字段。
- **429/超时**：降低迭代并发、缩小图片与输出规模，根据错误退避，避免整批无限重跑。

## 官方资料

- [Make HTTP 模块](https://apps.make.com/http)
- [Make 文本与二进制函数](https://help.make.com/text-and-binary-functions)
- [Gemini GenerateContent API](https://ai.google.dev/api/generate-content)
- [Gemini 图片理解指南](https://ai.google.dev/gemini-api/docs/image-understanding)
- [Gkotta Gemini API](/gemini-api)
