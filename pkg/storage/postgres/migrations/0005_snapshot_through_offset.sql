ALTER TABLE {{schema}}.document_snapshots
    ADD COLUMN IF NOT EXISTS through_offset bigint NOT NULL DEFAULT 0;

ALTER TABLE {{schema}}.document_snapshots
    DROP CONSTRAINT IF EXISTS document_snapshots_through_offset_nonnegative;

ALTER TABLE {{schema}}.document_snapshots
    ADD CONSTRAINT document_snapshots_through_offset_nonnegative
    CHECK (through_offset >= 0);
