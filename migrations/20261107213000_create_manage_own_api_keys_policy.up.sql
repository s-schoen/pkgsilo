-- The editable policy that lets every identity manage its own API keys and nobody else's. Reading them also
-- needs reading the identity, which the read-own-identity policy grants. A policy that already has this
-- policy's name stays as it is, and the new policy takes a name of its own instead.
INSERT INTO policies (id, name, source, created_by, updated_by, created_at, updated_at)
SELECT
    'eec871a1-b2f0-4e80-ad5b-edfe91e7e1a1',
    CASE
        WHEN EXISTS (SELECT 1 FROM policies WHERE name = 'manage-own-api-keys')
            THEN 'manage-own-api-keys-eec871a1-b2f0-4e80-ad5b-edfe91e7e1a1'
        ELSE 'manage-own-api-keys'
    END,
    'permit (
    principal,
    action in [Action::"apikey:read", Action::"apikey:create", Action::"apikey:delete"],
    resource is APIKey
) when { resource.owner == principal };',
    '00000000-0000-0000-0000-000000000000',
    '00000000-0000-0000-0000-000000000000',
    now(),
    now();

-- Every instance loads the policies again on its next request.
UPDATE policy_revision SET revision = revision + 1;
