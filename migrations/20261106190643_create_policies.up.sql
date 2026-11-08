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

-- The administrators group and the policy that grants it everything are created on the first start, together
-- with the first identity.
INSERT INTO policies (id, name, source, created_by, updated_by, created_at, updated_at)
VALUES (
    'b52e0d47-6c19-4f3a-8e75-2a9c14d7e608',
    'read-own-identity',
    'permit (principal, action == Action::"identity:read", resource is Identity) when { resource == principal };',
    '00000000-0000-0000-0000-000000000000',
    '00000000-0000-0000-0000-000000000000',
    now(),
    now()
);
