ALTER TABLE grsai_settlements
  ADD COLUMN public_task_id varchar(64),
  ADD COLUMN delivery_mode varchar(16) NOT NULL DEFAULT 'json',
  ADD COLUMN progress integer NOT NULL DEFAULT 0 CHECK (progress BETWEEN 0 AND 100),
  ADD COLUMN result_urls jsonb NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN hold_amount double precision NOT NULL DEFAULT 0,
  ADD COLUMN hold_state varchar(16) NOT NULL DEFAULT 'none',
  ADD COLUMN payload_delete_after timestamptz,
  ADD COLUMN expires_at timestamptz;
CREATE UNIQUE INDEX grsai_settlements_public_task_id_uq
  ON grsai_settlements (public_task_id) WHERE public_task_id IS NOT NULL;
CREATE INDEX grsai_settlements_owner_public_lookup_idx
  ON grsai_settlements (user_id, api_key_id, public_task_id);
CREATE INDEX grsai_settlements_owner_upstream_lookup_idx
  ON grsai_settlements (user_id, api_key_id, upstream_task_id) WHERE upstream_task_id IS NOT NULL;

CREATE TABLE grsai_task_payloads (
  settlement_id bigint PRIMARY KEY REFERENCES grsai_settlements(id) ON DELETE CASCADE,
  ciphertext text NOT NULL,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT NOW(),
  updated_at timestamptz NOT NULL DEFAULT NOW()
);
CREATE INDEX grsai_task_payloads_expires_at_idx ON grsai_task_payloads (expires_at);
