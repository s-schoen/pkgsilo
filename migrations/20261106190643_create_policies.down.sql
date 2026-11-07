-- Its memberships go with it, by the foreign key's cascade.
DELETE FROM groups WHERE id = '7c1d3e52-8a4f-4b6e-9f21-5d0c8e7a3b14';

DROP TABLE policy_revision;
DROP TABLE policies;
