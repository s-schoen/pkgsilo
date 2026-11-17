DELETE FROM policies WHERE id = 'aa16e382-ce94-4c7b-b40e-136864f67c29';

-- Every instance loads the policies again on its next request.
UPDATE policy_revision SET revision = revision + 1;
