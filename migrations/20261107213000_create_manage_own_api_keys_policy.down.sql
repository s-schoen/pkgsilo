DELETE FROM policies WHERE id = 'eec871a1-b2f0-4e80-ad5b-edfe91e7e1a1';

-- Every instance loads the policies again on its next request.
UPDATE policy_revision SET revision = revision + 1;
