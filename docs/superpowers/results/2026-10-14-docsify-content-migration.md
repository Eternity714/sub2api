# Docsify 固定生产正文目录迁移

日期：2026-10-14（Asia/Shanghai）

状态：**固定目录迁移及生产晋升已完成。** 完整浏览器验收、10%→50%→100% 公网灰度、338 秒观察和晋升后最终复核均通过。

用户要求生产 Markdown 目录固定为 `/opt/sub2api/content`。该路径已创建为真实目录，未使用符号链接，容器只读挂载到 `/usr/share/nginx/html/docs`。

## 内容保留与发布语义

首次从已上线 blue 稳定槽位复制维护正文，包含此前运行时修改，未使用镜像种子覆盖。迁移核对全部 46 个公开文件的相对路径及 SHA-256 完全一致；逐文件通过 green 容器 HTTP 读出并核对同一 SHA-256，证明实际容器用户可读。

固定目录 inode 为 `2247245`。green 容器 `sub2api-docsify-green-sha-411025133ebf4728eba0700bc4af959c128c1df9` 的启动时间为 `2026-10-14T11:59:53.806205439+08:00`。实际 Podman mount 的 Type 为 `bind`、Source 为 `/opt/sub2api/content`、Destination 为 `/usr/share/nginx/html/docs`、RW 为 `false`。

通过 SSH 隧道直连 green，在浏览器先读取临时正文 Initial，再明确修改 canonical 中对应文件为 Revised，刷新后 DOM 出现 Revised 且旧正文消失。更新后容器启动时间和目录 inode 均保持上述值。全部 10 项浏览器检查通过，包括导航、正文搜索、真实剪贴板复制、深链接刷新、Mermaid SVG、移动布局、挂载刷新和同域资源请求。

后续镜像发布只验证和复用已有正文，不覆盖文件、不调整已有权限、不更换目录或文件 inode。新增文章必须由维护者明确同步到固定目录，源码同时更新 `frontend/docs/`；更新单个文件后刷新即可，无需编译、重建镜像或重启容器。已有目录缺少必需入口、导航或包含维护文件、隐藏项、符号链接及特殊文件时拒绝发布。

旧 blue 容器和版本正文保留。迁移期可用 `gray-set-docsify-traffic.sh 0` 回切旧稳定入口；旧路径候选不能放量或晋升。脚本同时核对状态字段与实际 Podman 只读 bind mount，避免旧候选同 SHA 重用假成功。

## 本地验证

所有本地验证通过 Docker Compose 容器执行。灰度测试先在旧实现上出现预期失败，实现后最终 44 项保护回归和 5 个真实 Nginx 场景通过，`bash -n` 和 `git diff --check` 通过。新增 24 项覆盖正文逐字节保留、目录及文件 inode、权限、后续镜像不合并正文、稳定正文优先、非法目录拒绝、旧槽位兼容与实际 Mount Source/RW 门禁。独立只读审查无阻断问题。

本次发现验收工具的异步负例将“250ms 内至少轮询两次”当作正确性要求，冷启动时可能误判。诊断未在五次自然采样中重现单轮，但用 300ms 异步 false 和 250ms deadline 证明正确拒绝也可能只有一轮。负例改为 Node 异步 false、pending 超时和 delayed false→true 两轮断言，保持对未 await Promise 的检测；精确源码负例连续 10 次通过，移除 await 的变异被捕获。真实浏览器刷新断言未削弱。

```bash
docker compose -f deploy/docker-compose.test.yml run --rm --no-deps frontend-test bash -c 'set -euo pipefail; apt-get -qq update; apt-get -qq install -y --no-install-recommends nginx-light; bash -n deploy/gray-docsify-common.sh deploy/gray-deploy-docsify.sh deploy/gray-set-docsify-traffic.sh deploy/gray-promote-docsify.sh deploy/gray-docsify-status.sh deploy/tests/docsify-gray-test.sh; DOCSIFY_REAL_NGINX=/usr/sbin/nginx bash deploy/tests/docsify-gray-test.sh'
```

## 生产执行证据

复用已发布不可变镜像 `ghcr.io/eternity714/sub2api-docsify:sha-411025133ebf4728eba0700bc4af959c128c1df9`，未重建或发布新镜像。每次生产操作先运行主应用 `gray-status.sh`，通过已安装的文档灰度脚本部署、切流和晋升。

安装前的脚本及文档状态备份保存在 `/opt/sub2api/gray/docsify/backups/content-scripts.aChPqm`。候选部署于 `2026-10-14 11:59:55+08` 完成，green 端口 `127.0.0.1:18084`，stable blue、候选 0%、epoch 5，两个文档容器均 healthy。主机与容器 Nginx 校验通过，候选最近日志无 5xx 或 Nginx 错误。

`docsify-content-server-check.py` 在服务器核对真实目录、文件集合/bytes、实际挂载、逐文件 HTTP、健康、Nginx 和日志。`docsify-production-stage.py` 在 Compose 中验收 HTTPS 边缘；本次采用只存在于 canonical 的临时文章，分别核对 candidate/stable Cookie 的文章和原始 Markdown，证明真实上游去向。

| 阶段 | 已确认结果 |
| --- | --- |
| 0%，epoch 5 | 公网所有 Cookie 均进入旧稳定槽位，canonical 临时文章与原始 Markdown 均 404；候选直连全部正文和浏览器验证通过 |
| 10%，epoch 6 | `12:07:52+08` 核对通过；candidate Cookie 读到 Revised，stable Cookie 返回 404，主站健康与登录正常 |
| 50%，epoch 7 | `12:08:45+08` 核对通过；文章及原始 Markdown 的槽位探针、资源、主站检查通过 |
| 100%，epoch 8 | `docsify_full_traffic_at=1791950960`（`12:09:20+08`）；无 Cookie 请求读到 canonical Revised，显式 stable Cookie 仍走旧稳定槽位；阶段检查通过 |
| 观察与晋升 | `docsify_promoted_at=1791951298`（`12:14:58+08`）；累计 338 秒；稳定 green / 候选 blue / 候选比例 0%，epoch 9；green content 固定为 `/opt/sub2api/content` |
| 晋升后复核 | 公网 HTTP 10 项全部通过，耗时 48.600 秒；过期、stable、candidate Cookie 都读到 canonical 独有正文，旧资源真实 404；主站健康、登录及 CSP 正常 |

每阶段都确认主应用状态基线、Nginx、文档健康、完整正文可读及候选最近日志无 5xx 或非预期 Nginx 错误。100% HTTP 验收刻意请求 `/docs/_assets/no-such-runtime.js`，返回预期 404，并产生该精确路径的 Nginx alias ENOENT 日志；服务器检查仅识别此条负例，其它路径、方法、错误类型及所有 5xx 仍拒绝。日志分类经过独立只读复核。

100% 公网 HTTP 执行 10 项，9 项通过、1 项仅晋升后检查暂时跳过，耗时 42.752 秒。观察期第 186 秒、第 310 秒以及晋升后再次核对健康、46 文件读取、主应用基线、实际只读挂载和日志均通过。

临时浏览器文章已删除，Compose 公网检查确认文章和原始 Markdown 都返回 404。SSH 隧道已关闭。删除后再次核对 canonical 恢复为原 46 个公开文件、所有 bytes 一致、inode 和容器启动时间未变、Nginx 及日志通过，主应用两个状态文件仍匹配基线。旧 blue 容器、版本正文及全部灰度备份保留。

主应用稳定 green、候选 blue、候选流量 0%，两个容器 healthy。主应用文件 SHA-256 与首次上线基线一致：

```text
e46ed023f47403d054ea38267995d178f4d2862c1f6114d1477fcb7af563f819  /opt/sub2api/gray/state.env
5fb7225682a691a6f6fc83538160bcf59a5528bb6c5316d5afe474a932f0ed26  /opt/sub2api/gray/slots.env
```

## 维护入口

生产正文：`/opt/sub2api/content`；公开入口：[文档中心](https://www.gkotta.bid/docs/)。

维护前运行 `/opt/sub2api/scripts/gray-status.sh` 与 `gray-docsify-status.sh`，确认稳定槽位 content 指向固定目录。修改其中单个 Markdown、导航或图片后刷新；保持根目录不变，公开文件须可被 UID 101 读取。完整说明见 `frontend/docs/README.md`。
