-- GRS.AI 原生生图结算存储。仅保留结算所需的脱敏元数据；
-- 不保存 API key、提示词、图像内容或原始请求 JSON。

CREATE TABLE IF NOT EXISTS grsai_settlements (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL,
    group_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    api_key_id BIGINT NOT NULL,
    model VARCHAR(128) NOT NULL,
    base_unit_price DECIMAL(20,10) NOT NULL,
    group_rate_multiplier DECIMAL(10,4) NOT NULL,
    account_rate_multiplier DECIMAL(10,4) NOT NULL,
    billable_unit_price DECIMAL(20,10) NOT NULL,
    requested_image_count INTEGER NOT NULL,
    currency VARCHAR(16) NOT NULL DEFAULT 'USD',
    billing_idempotency_key VARCHAR(128) NOT NULL,
    upstream_task_id VARCHAR(255),
    upstream_status VARCHAR(32) NOT NULL DEFAULT 'not_submitted',
    internal_status VARCHAR(32) NOT NULL DEFAULT 'pending_upstream',
    retry_count INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL,
    last_error_summary VARCHAR(1024),
    settled_amount DECIMAL(20,10),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    upstream_bound_at TIMESTAMPTZ,
    result_updated_at TIMESTAMPTZ,
    settled_at TIMESTAMPTZ,
    closed_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS grsai_settlements_billing_idempotency_key_uq
    ON grsai_settlements (billing_idempotency_key);

CREATE UNIQUE INDEX IF NOT EXISTS grsai_settlements_account_upstream_task_uq
    ON grsai_settlements (account_id, upstream_task_id)
    WHERE upstream_task_id IS NOT NULL AND upstream_task_id <> '';

CREATE INDEX IF NOT EXISTS grsai_settlements_due_claim_idx
    ON grsai_settlements (internal_status, next_attempt_at)
    WHERE internal_status IN ('pending_upstream', 'pending_settlement', 'processing');

CREATE INDEX IF NOT EXISTS grsai_settlements_user_created_at_idx
    ON grsai_settlements (user_id, created_at);

COMMENT ON TABLE grsai_settlements IS 'GRS.AI 原生生图的脱敏结算状态；不存储密钥、提示词、图像或原始请求';
COMMENT ON COLUMN grsai_settlements.billing_idempotency_key IS '内部计费幂等键，全局唯一';
COMMENT ON COLUMN grsai_settlements.last_error_summary IS '脱敏后的最终错误摘要';
