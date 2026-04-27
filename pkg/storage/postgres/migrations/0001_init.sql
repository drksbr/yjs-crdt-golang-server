CREATE TABLE IF NOT EXISTS {{schema}}.schema_migrations (
    version bigint PRIMARY KEY,
    name text NOT NULL,
    applied_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS {{schema}}.document_snapshots (
    namespace text NOT NULL DEFAULT '',
    document_id text NOT NULL,
    snapshot_v1 bytea NOT NULL,
    stored_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (namespace, document_id)
);

CREATE INDEX IF NOT EXISTS document_snapshots_stored_at_idx
    ON {{schema}}.document_snapshots (stored_at DESC);
