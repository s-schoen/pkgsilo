-- A policy is a named document of one or more Cedar statements.
CREATE TABLE policies (
    id         uuid        PRIMARY KEY,
    name       text        NOT NULL,
    source     text        NOT NULL,
    -- Audit columns reference identities without a foreign key, so they survive deletes.
    created_by uuid        NOT NULL,
    updated_by uuid        NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT policies_name_key UNIQUE (name)
);

CREATE INDEX policies_created_at_id_idx ON policies (created_at, id);

-- The one row that says which version of the policies is current. Every policy write updates it in its
-- transaction, which serializes policy writes and tells every instance to load the policies again.
CREATE TABLE policy_revision (
    singleton boolean PRIMARY KEY DEFAULT true,
    revision  bigint  NOT NULL,
    CONSTRAINT policy_revision_singleton_check CHECK (singleton)
);

INSERT INTO policy_revision (revision) VALUES (1);

-- The group that the administrators policy grants everything. Policies reference groups by ID, so a group
-- that already has this group's name gets no access from it and stays as it is: the new group takes a name
-- of its own instead.
INSERT INTO groups (id, name, created_by, updated_by, created_at, updated_at)
SELECT
    '7c1d3e52-8a4f-4b6e-9f21-5d0c8e7a3b14',
    CASE
        WHEN EXISTS (SELECT 1 FROM groups WHERE name = 'administrators')
            THEN 'administrators-7c1d3e52-8a4f-4b6e-9f21-5d0c8e7a3b14'
        ELSE 'administrators'
    END,
    '00000000-0000-0000-0000-000000000000',
    '00000000-0000-0000-0000-000000000000',
    now(),
    now();

-- Without this, an existing database would lock everyone out. The first start's identity is the only one
-- created by the system actor, whatever it's called by now. The username would miss a renamed admin, and could
-- promote an unrelated identity that took the name later. If that identity was deleted, the group stays empty:
-- nothing tells who should inherit its access, so recovery is an operator's change in the database.
INSERT INTO group_members (group_id, identity_id, created_by, created_at)
SELECT
    '7c1d3e52-8a4f-4b6e-9f21-5d0c8e7a3b14',
    id,
    '00000000-0000-0000-0000-000000000000',
    now()
FROM identities
WHERE created_by = '00000000-0000-0000-0000-000000000000';

INSERT INTO policies (id, name, source, created_by, updated_by, created_at, updated_at)
VALUES
    (
        '3f9a6c10-2b7e-4d83-a5c4-81e6f0d29b37',
        'administrators',
        'permit (principal in Group::"7c1d3e52-8a4f-4b6e-9f21-5d0c8e7a3b14", action, resource);',
        '00000000-0000-0000-0000-000000000000',
        '00000000-0000-0000-0000-000000000000',
        now(),
        now()
    ),
    (
        'b52e0d47-6c19-4f3a-8e75-2a9c14d7e608',
        'read-own-identity',
        'permit (principal, action == Action::"identity:read", resource is Identity) when { resource == principal };',
        '00000000-0000-0000-0000-000000000000',
        '00000000-0000-0000-0000-000000000000',
        now(),
        now()
    );
