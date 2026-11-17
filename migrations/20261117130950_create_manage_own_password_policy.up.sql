-- The editable policy that lets every identity see when its own password was set and change it by giving
-- the current one. It grants no reset, so nobody can give themselves a first password: a leaked API key
-- can't create a password that would outlive its revocation. Reading the password also needs reading the
-- identity, which the read-own-identity policy grants. A policy that already has this policy's name stays
-- as it is, and the new policy takes a name of its own instead.
INSERT INTO policies (id, name, source, created_by, updated_by, created_at, updated_at)
SELECT
    'aa16e382-ce94-4c7b-b40e-136864f67c29',
    CASE
        WHEN EXISTS (SELECT 1 FROM policies WHERE name = 'manage-own-password')
            THEN 'manage-own-password-aa16e382-ce94-4c7b-b40e-136864f67c29'
        ELSE 'manage-own-password'
    END,
    'permit (
    principal,
    action in [Action::"password:read", Action::"password:readFields", Action::"password:change"],
    resource is Password
) when { resource.owner == principal };',
    '00000000-0000-0000-0000-000000000000',
    '00000000-0000-0000-0000-000000000000',
    now(),
    now();

-- Every instance loads the policies again on its next request.
UPDATE policy_revision SET revision = revision + 1;
