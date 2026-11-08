-- Reading a field of an object needs an action of its own now, such as identity:read:username, so the
-- self-service policies grant the fields of your own identity and API keys too. A policy that an admin has
-- edited since it was seeded stays as it is: its new meaning is the admin's to decide.
UPDATE policies
SET
    source = 'permit (principal, action in [Action::"identity:read", Action::"identity:readFields"], resource is Identity) when { resource == principal };',
    updated_by = '00000000-0000-0000-0000-000000000000',
    updated_at = now()
WHERE id = 'b52e0d47-6c19-4f3a-8e75-2a9c14d7e608'
    AND source = 'permit (principal, action == Action::"identity:read", resource is Identity) when { resource == principal };';

UPDATE policies
SET
    source = 'permit (
    principal,
    action in [Action::"apikey:read", Action::"apikey:readFields", Action::"apikey:create", Action::"apikey:delete"],
    resource is APIKey
) when { resource.owner == principal };',
    updated_by = '00000000-0000-0000-0000-000000000000',
    updated_at = now()
WHERE id = 'eec871a1-b2f0-4e80-ad5b-edfe91e7e1a1'
    AND source = 'permit (
    principal,
    action in [Action::"apikey:read", Action::"apikey:create", Action::"apikey:delete"],
    resource is APIKey
) when { resource.owner == principal };';

-- Every instance loads the policies again on its next request.
UPDATE policy_revision SET revision = revision + 1;
