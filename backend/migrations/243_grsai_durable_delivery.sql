-- Durable local task identity and v2 delivery metadata for native GRS.AI tasks.
-- All additions are nullable or defaulted so old settlement workers can keep
-- reading rows while a compatible version is rolled out.
ALTER TABLE grsai_settlements
    ADD COLUMN IF NOT EXISTS local_task_id VARCHAR(64),
    ADD COLUMN IF NOT EXISTS delivery_mode VARCHAR(16) NOT NULL DEFAULT 'json',
    ADD COLUMN IF NOT EXISTS public_status VARCHAR(32) NOT NULL DEFAULT 'queued',
    ADD COLUMN IF NOT EXISTS progress INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS result_json JSONB,
    ADD COLUMN IF NOT EXISTS link_expires_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS image_object_metadata JSONB,
    ADD COLUMN IF NOT EXISTS task_version INTEGER NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS submission_attempt INTEGER NOT NULL DEFAULT 0;

CREATE UNIQUE INDEX IF NOT EXISTS grsai_settlements_local_task_id_uq
    ON grsai_settlements (local_task_id)
    WHERE local_task_id IS NOT NULL AND local_task_id <> '';

CREATE INDEX IF NOT EXISTS grsai_settlements_api_key_created_at_idx
    ON grsai_settlements (user_id, api_key_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS grsai_settlements_v2_due_idx
    ON grsai_settlements (internal_status, next_attempt_at, id)
    WHERE internal_status LIKE 'v2_%';

CREATE TABLE IF NOT EXISTS grsai_task_payloads (
    local_task_id VARCHAR(64) PRIMARY KEY,
    payload_ciphertext TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS grsai_task_payloads_expires_at_idx
    ON grsai_task_payloads (expires_at);

COMMENT ON COLUMN grsai_settlements.local_task_id IS '下游可见的本地任务 ID；上游任务 ID 不对外暴露';
COMMENT ON COLUMN grsai_settlements.task_version IS '1 为旧结算链路，2 为持久交付链路';
COMMENT ON TABLE grsai_task_payloads IS 'GRS.AI 持久任务的加密上游请求载荷，不保存明文';
