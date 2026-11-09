BEGIN;

ALTER TABLE attachments
DROP CONSTRAINT attachments_owner_id_fkey;

COMMIT;