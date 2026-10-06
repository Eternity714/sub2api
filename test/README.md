# 本地真实图片端到端测试

通过 Docker Compose 请求本地预览 `localhost:18081`，默认模型为 `nano-banana-2-lite`。首次运行提交一次真实生成，会消耗上游额度并按本地分组配置扣费。

## 准备

在预览后台准备 **GRS.AI API Key 账号**、GRS.AI 内部分组、明确的图片模型单价与有足够余额的测试用户。分组和渠道定价界面的分类显示“媒体 API”，保存的内部平台仍为 `grsai`。创建该分组的 **Sub2API 用户 Key**；生图脚本只使用此 Key。上游账号 Key 在预览后台配置。

18081 的配置与验收记录见 [实测结果](RESULTS.md)。初始化代码 `media-setup.mjs` 限定本地 18081，创建或复用“媒体原生测试”配置，并拒绝复用平台不匹配的同名配置；上一轮创建的同名 OpenAI 测试账号、分组和渠道设为停用，保留历史任务及账务记录。它读取已忽略的 `.dev/media-preview/upstream-test.env` 并生成 `media-test.env`，给测试用户授权新分组，并开启可用渠道及模型广场。初始化与生图分别运行，初始化不会发送生图请求：

```powershell
docker compose -f test/docker-compose.media.yml run --rm media-setup
```

初始化环境字段为 `SUB2API_ADMIN_KEY`、`UPSTREAM_BASE_URL`、`UPSTREAM_API_KEY`、`SUB2API_TEST_EMAIL`、`SUB2API_TEST_PASSWORD`、`SUB2API_TEST_MODEL`。凭证不写入测试源码。

创建已忽略的 `.dev/media-preview/media-test.env`：

```dotenv
SUB2API_TEST_KEY=<测试用户的Sub2API Key>
SUB2API_TEST_MODEL=nano-banana-2-lite
SUB2API_TEST_BASE_URL=http://host.docker.internal:18081
```

`SUB2API_TEST_BASE_URL` 填本地服务根地址；容器访问 Windows 宿主机使用 `host.docker.internal`。当前客户端向上游使用 `/v1/api/generate` 与 `/v1/api/result` 协议。上游接入使用明确的 GRS.AI 账号类型；普通 OpenAI 自定义 Base URL 账号不参与该原生协议调度。

管理员接口保留 `grsai` 供配置、调度和计费使用；用户与公共接口中的平台为 `media`，页面显示“媒体 API”。模型和售价照常展示。

## 执行与恢复

在仓库根目录运行，所有测试均在容器内执行：

```powershell
docker compose -f test/docker-compose.media.yml run --rm media-image-e2e
```

脚本先检查 `/health`，首次仅提交一次 async 生成，立即保存本地任务 ID，再轮询最多 10 分钟。成功后下载图片并校验 HTTP 状态、图片 Content-Type 与文件格式，再对同一任务重复查询，核对状态与 URL 一致。图片下载不携带 Sub2API Key。

图片和脱敏报告保存到 `test/results/`（已忽略），最新恢复记录为 `test/results/last-run.json`。JSON 与阶段输出不包含 Key、JWT、原始响应、完整媒体 URL 或上游签名参数，仅记录 URL 哈希。

再次运行会自动恢复最新任务，不重新提交。也可以明确指定已保存的本地任务 ID：

```powershell
docker compose -f test/docker-compose.media.yml run --rm `
  -e TASK_ID=media_00000000-0000-0000-0000-000000000000 media-image-e2e
```

请替换示例 ID，使用生成时的同一个 Sub2API Key。恢复查询无需上游 Key 或管理员 Key。如果曾提交但响应丢失、未取得 ID，脚本会拒绝再次提交；先在本地后台或任务列表核查。

从上一轮 OpenAI 测试配置切换到新的 GRS.AI 配置时，初始化会使用新分组和新 Key。旧 `last-run.json` 属于旧 Key，不能用新 Key 恢复；首次验证新配置请显式使用下面的 `--new-run`。后续默认运行恢复这次新任务。

确认需要创建另一个收费测试任务时，明确传入 `--new-run`，旧报告会保留：

```powershell
docker compose -f test/docker-compose.media.yml run --rm `
  media-image-e2e node media-image-e2e.mjs --new-run
```

该参数会重新提交并再次收费，不与 `TASK_ID` 同时使用。主脚本验证任务与图片，不使用管理员 Key。

## 配置、鉴权与账务

下面两条命令不提交生图请求。配置检查验证内部 GRS.AI 账号/分组/渠道的绑定与价格，登录普通用户检查渠道定价、模型广场、分组、Key、仪表盘等 JSON 不暴露上游平台或凭据，并确认媒体模型和售价仍在；同时验证结果接口的 401/404 行为。账务检查只读管理员接口，重复查询成功任务 3 次，再核对余额和用量不变：

```powershell
docker compose -f test/docker-compose.media.yml run --rm media-config-check
docker compose -f test/docker-compose.media.yml run --rm media-billing-check
```

配置检查读取忽略目录中的 `test-fixture.json`、`upstream-test.env` 与 `media-test.env`。账务检查通过 `media-test.env` 的 `SUB2API_ADMIN_KEY`、`SUB2API_TEST_USER_ID`、`SUB2API_TEST_API_KEY_ID` 读取账务；初始化保存生成前的余额基准，默认成功请求数 1、每张 0.01 USD。再次明确生成后，可通过环境 `SUB2API_TEST_INITIAL_BALANCE`、`SUB2API_TEST_EXPECTED_REQUESTS`、`SUB2API_TEST_EXPECTED_COST` 更新累计验收预期。所有报告均只写入忽略的 `test/results/`。

预览返回无签名的 localhost/127.0.0.1 媒体 URL 时，脚本在容器内将其访问主机改为 `host.docker.internal`，保留端口与路径。签名链接不改写；请给对象存储配置容器可以访问的地址。默认下载上限为每张 32 MiB，支持 PNG、JPEG、WebP、GIF。

本次定位并修复的环境问题：预览曾继承已失效的 `host.docker.internal:7897` 代理，上游生成成功后图片下载报 `proxyconnect ... connection refused`。已移除预览容器中的代理环境变量，恢复原任务完成下载和存储，未重新提交生图。遇到类似问题先检查代理、下载和对象存储，保留原任务 ID。
