CREATE TABLE IF NOT EXISTS unified_media_tasks (
    id VARCHAR(100) PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    api_key_id BIGINT NOT NULL REFERENCES api_keys(id),
    group_id BIGINT NOT NULL REFERENCES groups(id),
    model VARCHAR(200) NOT NULL,
    media_type VARCHAR(10) NOT NULL CHECK (media_type IN ('image','video')),
    status VARCHAR(20) NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','in_progress','completed')),
    requested_count INTEGER NOT NULL CHECK (requested_count > 0),
    unit_price NUMERIC(20,8) NOT NULL CHECK (unit_price >= 0),
    account_unit_price NUMERIC(20,8) NOT NULL CHECK (account_unit_price >= 0),
    held_amount NUMERIC(20,8) NOT NULL CHECK (held_amount >= 0),
    charged_amount NUMERIC(20,8) NOT NULL DEFAULT 0 CHECK (charged_amount >= 0 AND charged_amount <= held_amount),
    hold_status VARCHAR(20) NOT NULL DEFAULT 'reserved' CHECK (hold_status IN ('reserved','captured','released')),
    payload_ciphertext TEXT NOT NULL,
    units JSONB NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(units)='array'),
    claim_version BIGINT NOT NULL DEFAULT 0,
    lease_until TIMESTAMPTZ,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deadline TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT unified_media_task_completed CHECK ((status='completed')=(completed_at IS NOT NULL))
);
CREATE INDEX IF NOT EXISTS idx_unified_media_tasks_due ON unified_media_tasks(next_attempt_at,lease_until) WHERE status<>'completed';
CREATE INDEX IF NOT EXISTS idx_unified_media_tasks_owned ON unified_media_tasks(user_id,api_key_id,media_type,id);
CREATE INDEX IF NOT EXISTS idx_unified_media_tasks_reserved_key ON unified_media_tasks(api_key_id) WHERE hold_status='reserved';
CREATE TABLE IF NOT EXISTS unified_media_account_holds (
    task_id VARCHAR(100) NOT NULL REFERENCES unified_media_tasks(id),
    unit_index INTEGER NOT NULL CHECK (unit_index>=0),
    attempt INTEGER NOT NULL CHECK (attempt>0),
    account_id BIGINT NOT NULL REFERENCES accounts(id),
    amount NUMERIC(20,8) NOT NULL CHECK (amount>=0),
    status VARCHAR(20) NOT NULL DEFAULT 'reserved' CHECK (status IN ('reserved','consumed','released')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(task_id,unit_index,attempt)
);
CREATE INDEX IF NOT EXISTS idx_unified_media_account_holds_reserved ON unified_media_account_holds(account_id) WHERE status='reserved';

-- Media public aliases and provider model IDs share the existing 200-character route contract.
ALTER TABLE usage_logs ALTER COLUMN model TYPE VARCHAR(200);
ALTER TABLE usage_logs ALTER COLUMN requested_model TYPE VARCHAR(200);
ALTER TABLE usage_logs ALTER COLUMN upstream_model TYPE VARCHAR(200);
