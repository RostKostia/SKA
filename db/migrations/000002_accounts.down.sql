BEGIN;

DELETE FROM outbound_messages WHERE kind = 'invite';
ALTER TABLE outbound_messages DROP CONSTRAINT outbound_messages_check1;
ALTER TABLE outbound_messages ADD CONSTRAINT outbound_messages_check1
    CHECK (kind IN ('owner_summary', 'budget') OR invoice_id IS NOT NULL);
ALTER TABLE outbound_messages DROP CONSTRAINT outbound_messages_kind_check;
ALTER TABLE outbound_messages ADD CONSTRAINT outbound_messages_kind_check
    CHECK (kind IN ('invoice', 'upcoming', 'overdue', 'receipt', 'owner_summary', 'budget'));
ALTER TABLE outbound_messages DROP COLUMN invite_id;

DROP TABLE IF EXISTS login_codes, oauth_states, invites;

-- Users without Google get a placeholder id again, so the old NOT NULL holds.
UPDATE users SET google_sub = 'local:' || id WHERE google_sub IS NULL;
ALTER TABLE users ALTER COLUMN google_sub SET NOT NULL;
ALTER TABLE users DROP COLUMN disabled_at, DROP COLUMN google_email, DROP COLUMN password_hash;

COMMIT;
