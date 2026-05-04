ALTER TABLE {{schema}}.document_update_logs
    ADD COLUMN IF NOT EXISTS update_v2 bytea;
