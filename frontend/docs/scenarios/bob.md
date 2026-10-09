# Bob 翻译

Bob 是 macOS 翻译工具。接入 Gkotta 需要安装支持自定义 API 的翻译插件，本文以开源 **OpenAI Translator for Bob** 为例，支持划词、输入翻译及 Bob 提供的截图翻译流程。

## 安装应用和插件

1. 从 [Bob 官网](https://bobtranslate.com/guide/)安装 Bob。当前 OpenAI Translator 插件要求 Bob 1.8.0 或更高版本，具体兼容范围以插件发布说明为准。
2. 从[插件发布页](https://github.com/nextai-translator/bob-plugin-openai-translator/releases/latest)下载 `openai-translator.bobplugin`，打开文件安装。
3. 在 Bob **设置/偏好设置 → 服务**中添加 OpenAI Translator 翻译服务。
4. 到 [Gkotta API 密钥](https://www.gkotta.bid/keys)创建密钥，确认[分组与模型](https://www.gkotta.bid/available-channels)支持 Chat Completions。

## 配置 Gkotta

在插件服务配置中填写：

| 字段 | 值 |
| --- | --- |
| API Key | `sk-YOUR_API_KEY` |
| 模型 | 选择「自定义模型」，填写 `YOUR_MODEL_ID` |
| API URL | `https://www.gkotta.bid/v1/chat/completions` |
| 推理设置 | 先保留「默认」 |

该插件的 **API URL 是完整请求地址**，不要只填写域名或 `/v1`。当前插件根据 `/chat/completions` 或 `/responses` 后缀决定请求格式；若要使用 Responses，应先确认当前模型与分组支持，再改为 `https://www.gkotta.bid/v1/responses`。

保存配置，启用该翻译服务。如果所安装插件版本的字段不同，按照该版本的配置手册设置，不能把其他 Bob 插件的字段规则混用。

## 验证翻译流程

先在 Bob 输入翻译窗口粘贴 `The connection is ready.`，目标语言选中文，选择刚添加的服务，确认得到正常译文。再选中网页的一句话，用 Bob 设置的划词快捷键翻译，并到[使用记录](https://www.gkotta.bid/usage)确认模型和消耗。

截图翻译先由 Bob 的 OCR 服务识别文字，再把文字交给翻译服务。需按 Bob 说明配置 OCR 并授予屏幕录制权限；不应把截图功能当作该聊天模型自动拥有的视觉能力。

## 调整技术文档翻译

当前插件的用户指令支持 `$text`、`$sourceLang`、`$targetLang` 变量。可保留系统指令的翻译用途，将用户指令改为：

```text
把以下内容翻译成 $targetLang，保留技术术语的英文原文和 Markdown 格式：

$text
```

先用一小段技术文本验证变量替换和译文格式，再翻译长文档。快捷键在 Bob 的设置中调整，避免与 macOS 或其他应用冲突。长文本分段处理，费用到 Gkotta 使用记录核对。

## 常见问题

- **API URL 格式不正确**：使用完整 `/v1/chat/completions` 或支持的 `/v1/responses` 地址，按插件版本要求检查结尾。
- **仍调用官方模型或地址**：确认选择了自定义模型，API URL 已保存，并在翻译窗口选择正确服务。
- **翻译失败/401**：检查密钥、实际模型 ID、分组和余额，不要把 ChatGPT 订阅或 Bob 授权码当作 API Key。
- **划词没有响应**：检查快捷键冲突和 Bob 的辅助功能权限，先验证输入翻译是否成功。
- **截图无文字**：排查屏幕录制权限及 OCR 服务，分别验证识别和翻译两个阶段。
- **长文响应慢或断开**：缩短文本，新建短翻译测试，并确认流式支持；不要持续重复提交同一长任务。

## 官方与插件资料

- [Bob 使用指南](https://bobtranslate.com/guide/)
- [OpenAI Translator 插件仓库](https://github.com/nextai-translator/bob-plugin-openai-translator)
- [插件配置手册](https://github.com/nextai-translator/bob-plugin-openai-translator/blob/main/docs/configuration_manual_CN.md)
- [Gkotta OpenAI API](/openai-api)
