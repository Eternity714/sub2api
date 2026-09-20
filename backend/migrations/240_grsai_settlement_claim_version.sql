-- 为已应用 GRS.AI 结算基础表的数据库补充单调领取版本，用于阻止过期 worker 写入。

ALTER TABLE grsai_settlements
    ADD COLUMN IF NOT EXISTS claim_version BIGINT NOT NULL DEFAULT 0;
