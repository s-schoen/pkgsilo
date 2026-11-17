-- A session keeps a browser logged in as its identity: deleting the identity removes its sessions.
CREATE TABLE sessions (
    id           uuid        PRIMARY KEY,
    identity_id  uuid        NOT NULL,
    -- The SHA-256 of the session's token, by which a request's cookie is looked up. The token itself is not
    -- stored.
    token_hash   bytea       NOT NULL,
    created_at   timestamptz NOT NULL,
    -- When the session was last used, written at most once a minute. The session ends when it's idle for
    -- too long.
    last_seen_at timestamptz NOT NULL,
    -- When the session ends at the latest, however often it's used.
    expires_at   timestamptz NOT NULL,
    -- The client's IP address and User-Agent at login. NULL: the IP address was unknown.
    client_ip    inet,
    user_agent   text        NOT NULL,
    CONSTRAINT sessions_token_hash_key UNIQUE (token_hash),
    CONSTRAINT sessions_identity_id_fkey
        FOREIGN KEY (identity_id) REFERENCES identities (id) ON DELETE CASCADE
);

-- Serves the cascade when an identity is deleted, and ending all sessions of an identity.
CREATE INDEX sessions_identity_id_idx ON sessions (identity_id);
