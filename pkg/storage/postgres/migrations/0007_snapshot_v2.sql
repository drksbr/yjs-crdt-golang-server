ALTER TABLE {{schema}}.document_snapshots
    ADD COLUMN IF NOT EXISTS snapshot_v2 bytea;
