ALTER TABLE grsai_settlements
  ADD COLUMN async_started_at timestamptz;

-- Rows accepted before this migration that already reached upstream are active.
UPDATE grsai_settlements
SET async_started_at = created_at
WHERE delivery_mode = 'async'
  AND (upstream_status <> 'not_submitted' OR upstream_task_id IS NOT NULL);

CREATE INDEX grsai_settlements_async_user_capacity_idx
  ON grsai_settlements (user_id, async_started_at, internal_status)
  WHERE delivery_mode = 'async';
