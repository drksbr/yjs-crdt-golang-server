CREATE TABLE IF NOT EXISTS {{schema}}.document_update_log_heads (
    namespace text NOT NULL DEFAULT '',
    document_id text NOT NULL,
    next_offset bigint NOT NULL,
    PRIMARY KEY (namespace, document_id)
);

CREATE TABLE IF NOT EXISTS {{schema}}.document_update_logs (
    namespace text NOT NULL DEFAULT '',
    document_id text NOT NULL,
    log_offset bigint NOT NULL,
    update_v1 bytea NOT NULL,
    stored_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (namespace, document_id, log_offset)
);

CREATE INDEX IF NOT EXISTS document_update_logs_stored_at_idx
    ON {{schema}}.document_update_logs (stored_at DESC);

CREATE TABLE IF NOT EXISTS {{schema}}.document_placements (
    namespace text NOT NULL DEFAULT '',
    document_id text NOT NULL,
    shard_id text NOT NULL,
    version bigint NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (namespace, document_id)
);

CREATE INDEX IF NOT EXISTS document_placements_shard_id_idx
    ON {{schema}}.document_placements (shard_id);

CREATE TABLE IF NOT EXISTS {{schema}}.shard_leases (
    shard_id text PRIMARY KEY,
    owner_node_id text NOT NULL,
    owner_epoch bigint NOT NULL,
    token text NOT NULL,
    acquired_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS shard_leases_expires_at_idx
    ON {{schema}}.shard_leases (expires_at);
