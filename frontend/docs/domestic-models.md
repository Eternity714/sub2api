# 国内模型接入

国内模型同样需要匹配模型 ID、密钥分组与接口协议。GLM 或 Qwen 的名称不会自动赋予某个分组模型权限，也不能用参考教程中的固定版本号推断本站当前可用模型。

## 先确认模型与协议

1. 在[可用渠道](https://www.gkotta.bid/available-channels)查看管理员开放的渠道和模型。
2. 在[API 密钥](https://www.gkotta.bid/keys)创建相应分组的密钥。
3. 查询该密钥的模型列表，并用所选协议发送最小请求。

```bash
export GKOTTA_API_KEY="sk-YOUR_API_KEY"
curl --fail-with-body "https://www.gkotta.bid/v1/models" \
  -H "Authorization: Bearer $GKOTTA_API_KEY"
```

下文 `YOUR_MODEL_ID` 使用实际返回或管理员确认的模型 ID。模型列表用于发现模型，生成时仍会检查额度、分组和上游能力。

## GLM

Gkotta 有智谱渠道能力。使用管理员配置的智谱分组时，按渠道说明选择它实际开放的模型及请求协议。GLM 原生能力和参数可查阅[智谱官方文档](https://docs.bigmodel.cn/)。

若该分组开放 OpenAI Chat Completions，客户端选择 OpenAI 兼容提供商，填写以下配置：

| 配置项 | 值 |
| --- | --- |
| Base URL | `https://www.gkotta.bid/v1` |
| API Key | 自己的 Gkotta 密钥 |
| Model | 该分组的实际 GLM 模型 ID |

先完成普通对话，再测试流式、推理和工具调用。不同 GLM 模型的参数和工具能力可能不同；编程客户端还需要所选模型满足它的工具调用要求。

## Qwen

管理员可以通过兼容上游配置 Qwen 等模型。只有当前密钥所属分组确实开放了对应模型，才能按[OpenAI API](/openai-api)接入。模型本身的能力可参考[阿里云百炼模型文档](https://help.aliyun.com/zh/model-studio/models)。

在[Cherry Studio](/scenarios/cherry-studio)、[Chatbox](/scenarios/chatbox)或[Open WebUI](/scenarios/open-webui)添加 OpenAI 兼容服务，地址填写 `https://www.gkotta.bid/v1`，手动添加该分组实际提供的 Qwen 模型 ID。

## 最小对话请求

macOS、Linux 或 Git Bash：

```bash
curl --fail-with-body "https://www.gkotta.bid/v1/chat/completions" \
  -H "Authorization: Bearer $GKOTTA_API_KEY" \
  -H "Content-Type: application/json" \
  --data '{"model":"YOUR_MODEL_ID","messages":[{"role":"user","content":"只回复 Hello"}]}'
```

Windows PowerShell：

```powershell
$env:GKOTTA_API_KEY = "sk-YOUR_API_KEY"
$body = @{
  model = "YOUR_MODEL_ID"
  messages = @(@{ role = "user"; content = "只回复 Hello" })
} | ConvertTo-Json -Depth 6
Invoke-RestMethod -Uri "https://www.gkotta.bid/v1/chat/completions" `
  -Method Post -Headers @{ Authorization = "Bearer $env:GKOTTA_API_KEY" } `
  -ContentType "application/json" -Body ([Text.Encoding]::UTF8.GetBytes($body))
```

在[使用记录](https://www.gkotta.bid/usage)核对实际模型、密钥和费用。出现 `model_not_found` 或分组错误时，优先核对模型 ID 与分组；不能通过修改显示名称获得未开放模型。没有工具调用能力的模型应先用于普通聊天，具体错误见[排错指南](/troubleshooting)。
