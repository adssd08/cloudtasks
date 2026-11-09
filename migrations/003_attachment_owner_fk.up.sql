BEGIN;

ALTER TABLE attachments
ADD CONSTRAINT attachments_owner_id_fkey
FOREIGN KEY (owner_id) REFERENCES users(id);

COMMIT;