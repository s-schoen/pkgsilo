CREATE TABLE groups (
    id         uuid        PRIMARY KEY,
    name       text        NOT NULL,
    -- Audit columns reference identities without a foreign key, so they survive deletes.
    created_by uuid        NOT NULL,
    updated_by uuid        NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT groups_name_key UNIQUE (name)
);

CREATE INDEX groups_created_at_id_idx ON groups (created_at, id);

-- A member belongs to its group and is an identity: deleting either removes the membership.
CREATE TABLE group_members (
    group_id    uuid        NOT NULL,
    identity_id uuid        NOT NULL,
    created_by  uuid        NOT NULL,
    created_at  timestamptz NOT NULL,
    CONSTRAINT group_members_pkey PRIMARY KEY (group_id, identity_id),
    CONSTRAINT group_members_group_id_fkey
        FOREIGN KEY (group_id) REFERENCES groups (id) ON DELETE CASCADE,
    CONSTRAINT group_members_identity_id_fkey
        FOREIGN KEY (identity_id) REFERENCES identities (id) ON DELETE CASCADE
);

CREATE INDEX group_members_group_id_created_at_identity_id_idx ON group_members (group_id, created_at, identity_id);
-- Serves the cascade when an identity is deleted.
CREATE INDEX group_members_identity_id_idx ON group_members (identity_id);
