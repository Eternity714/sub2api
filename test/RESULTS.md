# 18081 原生媒体生图实测结果

2026-10-06，通过 Docker Compose 测试容器访问本地 `http://127.0.0.1:18081`，使用恢复后的 GRS.AI API Key 账号与 `nano-banana-2-lite` 模型，1K、1:1、async。本轮新提交一次真实生图，任务 ID 为 `media_ac6fb115-9db2-4366-8432-66ff01d07da0`。

## 当前测试配置

| 对象 | 名称 | ID / 配置 |
| --- | --- | --- |
| 账号 | 媒体原生测试上游 | 3；GRS.AI API Key；Base URL `https://grsaiapi.com/` |
| 分组 | 媒体原生测试分组 | 4；内部平台 `grsai`，余额分组，启用媒体生成 |
| 渠道 | 媒体原生测试渠道 | 2；绑定分组 4，限定测试模型 |
| 用户 | 媒体测试用户 | 2；`media-test@sub2api.local`，本轮生成前余额 9.99 USD |
| 调用 Key | 媒体原生端到端测试 | 2；绑定分组 4，预算 1 USD，7 天有效 |
| 模型价格 | nano-banana-2-lite | 图片计费 0.01 USD/张，倍率 1，仅供本地验收 |

管理员账号配置保留 GRS.AI；分组、渠道分类显示媒体 API。管理员接口保留内部 `grsai`，普通用户与公共接口输出 `media`。渠道定价和模型广场仍保留对应模型及图片价格。上游路由由 GRS.AI 原生客户端明确使用 `POST /v1/api/generate`、`GET /v1/api/result`，普通 OpenAI 自定义 Base URL 账号不参与该链路。

测试用户密码和 Sub2API Key 保存在 Git 忽略的 `.dev/media-preview/test-login.txt`、`media-test.env`；管理员登录资料在 `login.txt`。源码与报告不保存密钥。上一轮 OpenAI 测试账号 2、分组 3 和渠道 1 已停用，历史任务和账务保留。

## 验证结果

- 54 项配置与鉴权检查通过；覆盖管理员账号、分组、渠道、Key 绑定和价格，以及普通用户分组、Key 列表/详情、渠道、登录与匿名模型广场、个人信息、额度、仪表盘和用量记录。
- 用户与公共 JSON 不包含 `grsai`、`GRS.AI`、上游 Base URL 或账号凭据；媒体平台为 `media`，模型和每张 0.01 USD 的价格仍在。
- 无鉴权查询结果返回 401，未知任务返回 404。
- 新生成返回 202，任务依次为 `queued`、`running`、`succeeded`，约 35 秒完成本地交付。
- 成功下载 1 张 JPEG，548,147 字节；HTTP、Content-Type 和文件签名校验通过，图片可正常查看。
- 本轮余额 9.99 → 9.98 USD，新 Key 的用量记录恰好 1 条，`actual_cost: 0.01`、`image_count: 1`、`billing_mode: image`。
- 同一任务重复查询 3 次，余额与用量不变；默认恢复运行成功下载原图片，没有重新提交生成。
- Docker Compose 后端完整单元测试、前端 lint/类型检查/关键回归和应用镜像构建通过；本地应用、PostgreSQL、Redis、MinIO 健康。
- 浏览器确认匿名模型广场筛选显示“媒体 API”，测试模型和每张 0.01 USD 的价格正常；模型广场相关类型检查与 44 项回归通过。
- 最终容器重启后恢复原任务仍通过；余额保持 9.98 USD，新 Key 的用量仍只有 1 条。

脱敏报告位于 `test/results/config-report.json`、`billing-report.json`、`last-run.json`。初次生成报告为 `image-e2e-2026-10-06T09-03-07-345Z.json`，首次恢复报告为 `image-e2e-2026-10-06T09-05-14-990Z.json`，最终容器重启后的恢复报告为 `image-e2e-2026-10-06T09-09-27-932Z.json`。图片为 `test/results/media_ac6fb115-9db2-4366-8432-66ff01d07da0-image-1.jpg`；公开页面截图为 [media-model-plaza.png](results/media-model-plaza.png)。

## 卡点与处理

本轮初始化脚本停用旧渠道时发现渠道状态使用 `disabled`，账号和分组使用 `inactive`。已修正脚本后复跑成功，复用已创建的新配置，没有重复创建或重置余额。

本机遗留 `BUILDX_BUILDER=mirror-builder` 指向不存在的构建器，已在构建命令中临时选择可用的 `default` 构建器，未改全局设置。

浏览器验收发现模型广场筛选直接显示 API 值 `media`。已补齐该页面的媒体平台翻译，保留筛选值不变，并重新构建、更新容器和核对页面。

上一轮已定位并移除预览容器的失效代理 `host.docker.internal:7897`：当时上游成功后图片下载卡住，修正代理并恢复原任务后完成交付。本轮新的原生媒体任务下载和存储正常，未再出现该卡点。

上一轮旧任务 `media_8841692e-8b6c-4ce3-9834-d464b8571ff9` 与余额 10 → 9.99 的记录仅作为历史。本报告的通过证据来自新的 GRS.AI 账号与新 Key。

人工入口为 `http://127.0.0.1:18081/login`，匿名模型广场为 `/model-plaza`。操作说明见 [README](README.md) 和本地 `.dev/media-preview/manual-test.md`。本次真实验证图片链路；视频接口保留，但未发起收费视频生成。
