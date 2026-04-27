ALTER TABLE IF EXISTS {{schema}}.shard_lease_generations
    DROP CONSTRAINT IF EXISTS shard_lease_generations_last_epoch_check;

ALTER TABLE IF EXISTS {{schema}}.shard_lease_generations
    ADD CONSTRAINT shard_lease_generations_last_epoch_check CHECK (last_epoch >= 0);
