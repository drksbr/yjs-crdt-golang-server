ALTER TABLE {{schema}}.document_snapshots
    ADD COLUMN IF NOT EXISTS owner_epoch bigint NOT NULL DEFAULT 0;

ALTER TABLE {{schema}}.document_snapshots
    DROP CONSTRAINT IF EXISTS document_snapshots_owner_epoch_nonnegative;

ALTER TABLE {{schema}}.document_snapshots
    ADD CONSTRAINT document_snapshots_owner_epoch_nonnegative
    CHECK (owner_epoch >= 0);

ALTER TABLE {{schema}}.document_update_logs
    ADD COLUMN IF NOT EXISTS owner_epoch bigint NOT NULL DEFAULT 0;

ALTER TABLE {{schema}}.document_update_logs
    DROP CONSTRAINT IF EXISTS document_update_logs_owner_epoch_nonnegative;

ALTER TABLE {{schema}}.document_update_logs
    ADD CONSTRAINT document_update_logs_owner_epoch_nonnegative
    CHECK (owner_epoch >= 0);
