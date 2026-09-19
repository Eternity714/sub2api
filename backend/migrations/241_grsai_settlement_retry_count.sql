-- 将结算失败次数与上游轮询领取次数分离，保证结算退避不受长任务轮询影响。

ALTER TABLE grsai_settlements
    ADD COLUMN IF NOT EXISTS settlement_retry_count INTEGER NOT NULL DEFAULT 0;

COMMENT ON COLUMN grsai_settlements.settlement_retry_count
    IS '仅统计结算扣费失败次数；不包含上游结果查询或领取次数';
