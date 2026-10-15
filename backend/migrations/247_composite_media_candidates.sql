-- Keep existing non-media aliases unique; media aliases have ordered provider/model candidates.
ALTER TABLE composite_model_routes
    ADD COLUMN IF NOT EXISTS capability_model VARCHAR(200) NOT NULL DEFAULT '';

ALTER TABLE composite_model_routes
    DROP CONSTRAINT IF EXISTS composite_model_routes_endpoint_check;
ALTER TABLE composite_model_routes
    ADD CONSTRAINT composite_model_routes_endpoint_check
    CHECK (endpoint IN ('any', 'messages', 'count_tokens', 'responses', 'chat_completions', 'embeddings', 'images', 'videos', 'gemini'));

DROP INDEX IF EXISTS idx_composite_model_routes_unique_active;
CREATE UNIQUE INDEX IF NOT EXISTS idx_composite_model_routes_unique_legacy_active
    ON composite_model_routes (group_id, endpoint, match_type, public_model)
    WHERE deleted_at IS NULL AND endpoint NOT IN ('images', 'videos');
CREATE UNIQUE INDEX IF NOT EXISTS idx_composite_model_routes_unique_media_active
    ON composite_model_routes (group_id, endpoint, match_type, public_model, target_platform, upstream_model)
    WHERE deleted_at IS NULL AND endpoint IN ('images', 'videos');
