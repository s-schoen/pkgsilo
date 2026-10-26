CREATE TABLE identities (
    id           uuid        PRIMARY KEY,
    username     text        NOT NULL,
    display_name text        NOT NULL,
    -- Audit columns reference identities without a foreign key, so they survive deletes.
    created_by   uuid        NOT NULL,
    updated_by   uuid        NOT NULL,
    created_at   timestamptz NOT NULL,
    updated_at   timestamptz NOT NULL,
    CONSTRAINT identities_username_key UNIQUE (username)
);

CREATE INDEX identities_created_at_id_idx ON identities (created_at, id);
