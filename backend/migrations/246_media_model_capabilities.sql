CREATE TABLE IF NOT EXISTS media_model_capabilities (
    id BIGSERIAL PRIMARY KEY,
    public_model VARCHAR(200) NOT NULL CHECK (char_length(btrim(public_model)) > 0),
    media_type VARCHAR(10) NOT NULL CHECK (media_type IN ('image', 'video')),
    schema_version INTEGER NOT NULL DEFAULT 1 CHECK (schema_version = 1),
    revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0),
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    schema JSONB NOT NULL CHECK (jsonb_typeof(schema) = 'object'),
    template_id VARCHAR(100),
    template_version INTEGER CHECK (template_version > 0),
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT media_model_capabilities_template_pair CHECK ((template_id IS NULL) = (template_version IS NULL))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_media_model_capabilities_unique_active
    ON media_model_capabilities (public_model, media_type)
    WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_media_model_capabilities_type_enabled_active
    ON media_model_capabilities (media_type, enabled, id)
    WHERE deleted_at IS NULL;
