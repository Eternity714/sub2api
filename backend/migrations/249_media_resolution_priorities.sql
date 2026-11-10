-- Image providers can have a different fallback order for each resolution tier.
-- Existing binaries and routes retain their base priority during gray rollout.
ALTER TABLE composite_model_routes
    ADD COLUMN IF NOT EXISTS resolution_priorities JSONB NOT NULL DEFAULT '{}'::jsonb;
