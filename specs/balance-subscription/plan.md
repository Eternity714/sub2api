# 实现方案

在现有 documentation-preview 隔离 worktree 中实现，基线提交 79bf4d27aa07960227cad11a5fc890e5316f172d。

- 新增已认证 POST /api/v1/payment/orders/balance-subscription，输入 plan_id 和 UUID idempotency_key；身份由 JWT 取得。前端附带 expected_amount，仅核对确认报价，实际价格仍由服务器决定；价格不一致时零扣款并要求重新确认。
- checkout-info 添加 subscription_balance_enabled，旧响应缺字段时前端按 false 处理。支付总开关和订阅开关决定能力；余额充值开关不控制余额消费。
- 用绑定用户和请求标识的确定性 out_trade_no 复用已有唯一约束，持久回放已完成订单；核对原套餐防止请求标识复用。
- 单个 Ent 事务锁定用户，复用严格 AdjustBalance、事务感知的订阅发放/续期，原子创建完成订单和扣款/发放审计。用户锁保护首次订阅不存在时的并发购买；续费沿用订阅行锁。
- USD 金额使用 decimal 两位数；订单 currency 快照为 USD。已有 balance 已扣除异步任务冻结金额，不再次减 frozen_balance。
- 提交后同步失效余额、身份及订阅缓存；完成结果不因缓存/页面刷新错误而失败。余额消费隔离外部回调、补发、退款和返佣；排除外部收款统计。
- 前端在套餐确认区显示余额方式、USD 扣款及剩余/不足，明确确认。同步完成绕过 QR/OAuth/第三方窗口。单次意图的 UUID 和确认报价按账户持久保存，不明确时锁定切换并重用；仅服务端核对后明确拒绝的错误才解除，通用鉴权/路由错误保留意图。
- CDK 原生链接放在页面共用底部，target=_blank，rel=noopener noreferrer。

验证使用 Docker Compose：服务/处理器单元、前端消费者和用户流程测试、真实 PostgreSQL Testcontainers 原子性与并发测试、独立审核以及本地真实浏览器验收。生产沿用服务器灰度脚本和不可变镜像，测试不消费真实资金、不调用付费模型。
