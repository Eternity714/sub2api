CREATE TABLE IF NOT EXISTS grsai_balance_holds (
    local_task_id TEXT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    api_key_id BIGINT NOT NULL,
    amount NUMERIC(20, 8) NOT NULL CHECK (amount >= 0),
    status TEXT NOT NULL CHECK (status IN ('reserved', 'captured', 'released')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS grsai_balance_holds_user_status_idx
    ON grsai_balance_holds (user_id, status);
