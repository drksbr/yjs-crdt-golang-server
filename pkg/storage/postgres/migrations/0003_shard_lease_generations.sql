CREATE TABLE IF NOT EXISTS {{schema}}.shard_lease_generations (
    shard_id text PRIMARY KEY,
    last_epoch bigint NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (last_epoch > 0)
);

INSERT INTO {{schema}}.shard_lease_generations (shard_id, last_epoch, updated_at)
SELECT shard_id, owner_epoch, now()
FROM {{schema}}.shard_leases
WHERE owner_epoch > 0
ON CONFLICT (shard_id)
DO UPDATE SET
    last_epoch = GREATEST({{schema}}.shard_lease_generations.last_epoch, EXCLUDED.last_epoch),
    updated_at = now();
