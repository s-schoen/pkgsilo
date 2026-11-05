-- An API key belongs to its identity: deleting the identity removes its keys.
CREATE TABLE api_keys (
    id           uuid        PRIMARY KEY,
    identity_id  uuid        NOT NULL,
    name         text        NOT NULL,
    -- The keyed hash of the key, by which a request's key is looked up. The key itself is not stored.
    key_hash     bytea       NOT NULL,
    -- The key's last characters, which tell the holder of a key which record it belongs to.
    key_suffix   text        NOT NULL,
    -- NULL: the key never expires.
    expires_at   timestamptz,
    -- NULL: the key has not been used yet.
    last_used_at timestamptz,
    -- The audit column references identities without a foreign key, so it survives deletes.
    created_by   uuid        NOT NULL,
    created_at   timestamptz NOT NULL,
    CONSTRAINT api_keys_key_hash_key UNIQUE (key_hash),
    CONSTRAINT api_keys_identity_id_fkey
        FOREIGN KEY (identity_id) REFERENCES identities (id) ON DELETE CASCADE
);

-- Serves listing an identity's keys, and the cascade when an identity is deleted.
CREATE INDEX api_keys_identity_id_created_at_id_idx ON api_keys (identity_id, created_at, id);
