-- An identity has at most one password, which belongs to it: deleting the identity removes its password.
CREATE TABLE passwords (
    identity_id uuid        PRIMARY KEY,
    -- The Argon2id hash of the password as a PHC string, with its parameters and salt. The password itself
    -- is not stored.
    hash        text        NOT NULL,
    -- When the password was last set. Rehashing it with new parameters doesn't change it.
    set_at      timestamptz NOT NULL,
    CONSTRAINT passwords_identity_id_fkey
        FOREIGN KEY (identity_id) REFERENCES identities (id) ON DELETE CASCADE
);
