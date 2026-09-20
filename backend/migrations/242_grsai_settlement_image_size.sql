-- Persist the normalized image-size tier required for durable GRS.AI usage logs.
-- Existing rows predate this metadata, so recovery uses the established 2K default.
ALTER TABLE grsai_settlements
    ADD COLUMN IF NOT EXISTS image_size VARCHAR(10) NOT NULL DEFAULT '2K';

ALTER TABLE grsai_settlements
    DROP CONSTRAINT IF EXISTS grsai_settlements_image_size_check;

ALTER TABLE grsai_settlements
    ADD CONSTRAINT grsai_settlements_image_size_check
    CHECK (image_size IN ('1K', '2K', '4K')) NOT VALID;
