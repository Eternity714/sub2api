-- Make an existing pre-242 image_size column safe before 242 validates it.
-- The letter suffix deliberately places this compatibility migration between
-- 241 and 242 without rewriting either checksum-protected migration.
ALTER TABLE grsai_settlements
    ADD COLUMN IF NOT EXISTS image_size VARCHAR(10);

ALTER TABLE grsai_settlements
    ALTER COLUMN image_size SET DEFAULT '2K';

UPDATE grsai_settlements
SET image_size = '2K'
WHERE image_size IS NULL
   OR image_size NOT IN ('1K', '2K', '4K');

ALTER TABLE grsai_settlements
    ALTER COLUMN image_size SET NOT NULL;
