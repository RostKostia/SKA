-- Kerdos accounts: sign in with email and password, or with Google. One business: the first
-- account becomes its owner, and the owner invites everyone else by email.
BEGIN;

-- A user signs in with a password, a Google account, or both.
ALTER TABLE users ALTER COLUMN google_sub DROP NOT NULL;
ALTER TABLE users
    ADD COLUMN password_hash text,         -- 'pbkdf2-sha256$<iterations>$<salt>$<hash>', never the password
    ADD COLUMN google_email  text,         -- the linked Google account's address, shown on the account page
    ADD COLUMN disabled_at   timestamptz;  -- removed by the owner: can't sign in, sessions deleted
-- The first seed used a placeholder Google id ('sample-admin'; the down migration writes
-- 'local:<id>'). Nobody can sign in with those, so drop them: the first account created then
-- takes over this owner row. Real Google ids are digits only.
UPDATE users SET google_sub = NULL WHERE google_sub = 'sample-admin' OR google_sub LIKE 'local:%';

-- An invitation to join the business. The link carries a random token; only its hash is stored.
CREATE TABLE invites (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email        text        NOT NULL,
    name         text,
    role         text        NOT NULL DEFAULT 'user' CHECK (role IN ('admin', 'user')),
    token_hash   bytea       NOT NULL UNIQUE,
    invited_by   uuid        REFERENCES users (id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz NOT NULL,
    accepted_at  timestamptz,
    accepted_by  uuid        REFERENCES users (id) ON DELETE SET NULL,
    revoked_at   timestamptz
);
-- At most one open invitation per address.
CREATE UNIQUE INDEX invites_open_email_key ON invites (lower(email)) WHERE accepted_at IS NULL AND revoked_at IS NULL;

-- Google sign-in in progress: the OAuth state (hashed), the PKCE verifier and where to return.
CREATE TABLE oauth_states (
    state_hash     bytea PRIMARY KEY,
    purpose        text        NOT NULL CHECK (purpose IN ('signin', 'link')),
    user_id        uuid        REFERENCES users (id) ON DELETE CASCADE,   -- link: the signed-in user
    invite_id      uuid        REFERENCES invites (id) ON DELETE CASCADE, -- signin from an invitation
    code_verifier  text        NOT NULL,
    return_to      text        NOT NULL,
    expires_at     timestamptz NOT NULL,
    CHECK ((purpose = 'link') = (user_id IS NOT NULL))
);

-- One-time codes that hand a finished Google sign-in to the frontend, which swaps the code
-- for a session (POST /api/auth/exchange). Hashed, single use, two minutes.
CREATE TABLE login_codes (
    code_hash   bytea PRIMARY KEY,
    user_id     uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at  timestamptz NOT NULL,
    used_at     timestamptz
);

-- Invitation emails go through the same send log and queue as every other email.
ALTER TABLE outbound_messages ADD COLUMN invite_id uuid REFERENCES invites (id) ON DELETE SET NULL;
ALTER TABLE outbound_messages DROP CONSTRAINT outbound_messages_kind_check;
ALTER TABLE outbound_messages ADD CONSTRAINT outbound_messages_kind_check
    CHECK (kind IN ('invoice', 'upcoming', 'overdue', 'receipt', 'owner_summary', 'budget', 'invite'));
ALTER TABLE outbound_messages DROP CONSTRAINT outbound_messages_check1;
ALTER TABLE outbound_messages ADD CONSTRAINT outbound_messages_check1
    CHECK (kind IN ('owner_summary', 'budget', 'invite') OR invoice_id IS NOT NULL);

COMMIT;
