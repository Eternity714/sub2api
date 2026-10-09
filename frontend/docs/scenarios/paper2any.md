# Paper2Any 论文工作流

Paper2Any 是论文、图表与演示文稿的开源工作流工具。它的 Simple 模式支持统一配置文本模型 API；可以将该入口接入 Gkotta 的 OpenAI Chat Completions。本文先完成文本链路，图像、PDF 解析和本地视觉服务按所用流程分别配置。

## 获取项目与配置模板

使用[Paper2Any 官方仓库](https://github.com/OpenDCAI/Paper2Any)。准备 Git、Docker 和项目要求的运行环境，在 Bash、WSL 或相应 Linux 环境执行：

```bash
git clone https://github.com/OpenDCAI/Paper2Any.git
cd Paper2Any
cp fastapi_app/.env.simple.example fastapi_app/.env
cp frontend-workflow/.env.simple.example frontend-workflow/.env
cp deploy/docker.env.example deploy/docker.env
```

使用当前仓库中的模板保留其他运行配置。不要直接复制第三方教程的完整旧版 env 覆盖模板。

## 配置服务端文本入口

在 `fastapi_app/.env` 中修改：

```dotenv
BACKEND_API_KEY=YOUR_PAPER2ANY_INTERNAL_KEY
APP_BILLING_MODE=free
PAPER2ANY_CONFIG_MODE=simple
SIMPLE_TEXT_API_URL=https://www.gkotta.bid/v1
SIMPLE_TEXT_API_KEY=sk-YOUR_API_KEY
SIMPLE_TEXT_MODEL=YOUR_MODEL_ID
```

`SIMPLE_TEXT_MODEL` 必须选择当前 Gkotta 分组支持 Chat Completions 的真实模型 ID。`APP_BILLING_MODE=free` 是 Paper2Any 自身的应用计费模式，经过 Gkotta 的模型调用仍按本站规则收费。

`BACKEND_API_KEY` 是 Paper2Any 前后端通信的内部凭证，与 Gkotta 密钥不同。在 `frontend-workflow/.env` 设置：

```dotenv
VITE_API_KEY=YOUR_PAPER2ANY_INTERNAL_KEY
VITE_API_BASE_URL=
VITE_DEFAULT_LLM_API_URL=https://www.gkotta.bid/v1
VITE_DEFAULT_LLM_MODEL=YOUR_MODEL_ID
```

两个内部凭证需要一致。Gkotta 密钥只放在服务端的 `SIMPLE_TEXT_API_KEY`，不要填入前端 `VITE_*` 变量。Paper2Any 的内部凭证会被前端使用；若将工具开放给其他用户，按其官方部署方案配置登录和访问保护。

## 启动与验证

按官方 Docker 部署入口启动：

```bash
bash deploy/docker-up.sh
```

默认前端为 `http://localhost:3000`，后端健康地址为 `http://localhost:8000/health`，实际端口以 `deploy/docker.env` 为准。

先选择 Paper2PPT 的文本/主题输入，输入“介绍 HTTP API 的基础概念，面向初学者，生成五页演示文稿的大纲”，只完成文本大纲阶段，在生图前停止。到 Gkotta [使用记录](https://www.gkotta.bid/usage)检查实际模型、密钥和消耗，确认统一文本配置生效。修改 env 后按官方说明重新构建，前端显示配置会在构建时写入。

## 图片和文档处理依赖

图片流程有独立的 `SIMPLE_IMAGE_API_URL`、`SIMPLE_IMAGE_API_KEY` 和 `SIMPLE_IMAGE_MODEL`。使用前需要核对所选 Paper2Any 图片适配器发送的是 Gemini 原生还是 OpenAI Images 格式，再按[图像生成 API](/image-generate)填写匹配地址和模型；文本 Base URL 不能直接作为所有图片适配器的配置。

部分流程还依赖 MinerU、OCR/VLM、SAM3、本地 GPU、Supabase 或向量服务。这些按[官方配置指南](https://github.com/OpenDCAI/Paper2Any/blob/main/docs/guides/configuration.md)单独准备。完成文本接入后，再逐项开启需要的流程。

## 排错

| 现象 | 检查 |
| --- | --- |
| Paper2Any 页面出现内部鉴权错误 | 核对 `VITE_API_KEY` 与 `BACKEND_API_KEY`，修改前端配置后重新构建 |
| Gkotta 返回 401 / 403 | 检查 `SIMPLE_TEXT_API_KEY`、密钥分组及限制 |
| 仍使用模板中的模型 | 确认 Simple 模式及 `SIMPLE_TEXT_MODEL`，检查流程是否有更高优先级覆盖 |
| 文本正常但 PDF、图片或导出失败 | 检查对应解析、图片、GPU和导出服务日志 |
| 404 / model_not_found | 检查 `/v1` 只出现一次、实际模型 ID和接口协议 |

官方安装和 Docker 说明见[Paper2Any README](https://github.com/OpenDCAI/Paper2Any)，本站额度和请求错误见[排错指南](/troubleshooting)。
