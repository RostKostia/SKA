-- Kerdos schema. Money is integer cents; dates are DATE; instants are timestamptz.
-- Single company: the company table holds exactly one row.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE company (
    id                boolean PRIMARY KEY DEFAULT true CHECK (id),
    name              text         NOT NULL,
    currency          char(3)      NOT NULL DEFAULT 'EUR',
    vat_rate_percent  numeric(4,2) NOT NULL DEFAULT 19.00 CHECK (vat_rate_percent BETWEEN 0 AND 100),
    invoice_prefix    text         NOT NULL DEFAULT 'KRD'
);

CREATE TABLE users (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    google_sub     text        NOT NULL UNIQUE,
    email          text        NOT NULL,
    name           text        NOT NULL,
    role           text        NOT NULL DEFAULT 'user' CHECK (role IN ('admin', 'user')),
    created_at     timestamptz NOT NULL DEFAULT now(),
    last_login_at  timestamptz
);
CREATE UNIQUE INDEX users_email_key ON users (lower(email));

-- Tokens are encrypted by the backend before they reach this table.
CREATE TABLE google_tokens (
    user_id             uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    refresh_token_enc   bytea       NOT NULL,
    access_token_enc    bytea,
    access_expires_at   timestamptz,
    scopes              text[]      NOT NULL,
    key_version         smallint    NOT NULL,
    updated_at          timestamptz NOT NULL DEFAULT now(),
    revoked_at          timestamptz
);

-- Opaque session tokens are stored as hashes only.
CREATE TABLE sessions (
    token_hash  bytea PRIMARY KEY,
    user_id     uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at  timestamptz NOT NULL DEFAULT now(),
    expires_at  timestamptz NOT NULL
);
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);

-- Clients and suppliers.
CREATE TABLE counterparties (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name        text        NOT NULL,
    email       text,
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (name, email)
);

CREATE TABLE invoice_counters (
    year        smallint PRIMARY KEY,
    last_value  integer NOT NULL
);

-- Next receivable invoice number, e.g. KRD-2026-0001. Atomic per year.
CREATE FUNCTION next_invoice_number(p_year smallint) RETURNS text
LANGUAGE sql AS $$
    WITH n AS (
        INSERT INTO invoice_counters (year, last_value) VALUES (p_year, 1)
        ON CONFLICT (year) DO UPDATE SET last_value = invoice_counters.last_value + 1
        RETURNING last_value
    )
    SELECT c.invoice_prefix || '-' || p_year || '-' || lpad(n.last_value::text, 4, '0')
    FROM n, company c;
$$;

CREATE TABLE invoices (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    number           text        NOT NULL,
    direction        text        NOT NULL CHECK (direction IN ('payable', 'receivable')),
    counterparty_id  uuid        NOT NULL REFERENCES counterparties (id),
    description      text,
    amount_cents     bigint      NOT NULL CHECK (amount_cents > 0),
    vat_cents        bigint      NOT NULL DEFAULT 0 CHECK (vat_cents >= 0),
    currency         char(3)     NOT NULL DEFAULT 'EUR',
    issue_date       date        NOT NULL,
    due_date         date        NOT NULL,
    status           text        NOT NULL DEFAULT 'draft'
                     CHECK (status IN ('draft', 'sent', 'paid', 'overdue', 'cancelled')),
    paid_at          timestamptz,
    source           text        NOT NULL DEFAULT 'manual' CHECK (source IN ('manual', 'jinius', 'chat', 'boc')),
    external_id      text,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    CHECK (due_date >= issue_date),
    CHECK ((status = 'paid') = (paid_at IS NOT NULL)),
    UNIQUE (direction, counterparty_id, number)
);
COMMENT ON COLUMN invoices.amount_cents IS 'Total including VAT';
COMMENT ON COLUMN invoices.vat_cents IS 'VAT portion contained in amount_cents';
CREATE INDEX invoices_list_idx ON invoices (direction, status, due_date);
CREATE INDEX invoices_counterparty_idx ON invoices (counterparty_id);
CREATE UNIQUE INDEX invoices_external_id_key ON invoices (source, external_id) WHERE external_id IS NOT NULL;

CREATE TABLE invoice_lines (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id        uuid          NOT NULL REFERENCES invoices (id) ON DELETE CASCADE,
    position          smallint      NOT NULL,
    description       text          NOT NULL,
    quantity          numeric(10,2) NOT NULL CHECK (quantity > 0),
    unit_price_cents  bigint        NOT NULL CHECK (unit_price_cents >= 0),
    UNIQUE (invoice_id, position)
);

-- Chat invoicing: "Invoice Kostas, 3 hours at 40" is parsed into a draft the user approves.
CREATE TABLE invoice_drafts (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    source_text text        NOT NULL,
    parsed      jsonb       NOT NULL,
    status      text        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'discarded')),
    invoice_id  uuid        REFERENCES invoices (id),
    created_by  uuid        NOT NULL REFERENCES users (id),
    created_at  timestamptz NOT NULL DEFAULT now(),
    CHECK ((status = 'approved') = (invoice_id IS NOT NULL))
);

CREATE TABLE bank_accounts (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    boc_account_id   text        NOT NULL UNIQUE,
    iban             text,
    currency         char(3)     NOT NULL DEFAULT 'EUR',
    balance_cents    bigint      NOT NULL,
    balance_at       timestamptz NOT NULL
);

-- BoC transactions: source of payment matching and of actual spend by category.
CREATE TABLE bank_transactions (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    bank_account_id      uuid        NOT NULL REFERENCES bank_accounts (id),
    boc_transaction_id   text        NOT NULL UNIQUE,
    booked_date          date        NOT NULL,
    amount_cents         bigint      NOT NULL, -- negative = money out
    currency             char(3)     NOT NULL DEFAULT 'EUR',
    description          text,
    counterparty_name    text,
    category             text,
    matched_invoice_id   uuid        REFERENCES invoices (id),
    raw                  jsonb,
    imported_at          timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX bank_transactions_booked_idx ON bank_transactions (booked_date);
CREATE INDEX bank_transactions_matched_idx ON bank_transactions (matched_invoice_id) WHERE matched_invoice_id IS NOT NULL;

CREATE TABLE budgets (
    month         date    NOT NULL CHECK (extract(day FROM month) = 1),
    category      text    NOT NULL,
    amount_cents  bigint  NOT NULL CHECK (amount_cents >= 0),
    PRIMARY KEY (month, category)
);

-- Templates use {client_name}, {invoice_number}, {amount}, {due_date}.
CREATE TABLE reminder_rules (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    kind         text        NOT NULL CHECK (kind IN ('upcoming', 'overdue', 'budget')),
    offsets_days integer[]   NOT NULL DEFAULT '{}', -- days before due (upcoming) or after due (overdue)
    subject      text        NOT NULL,
    body         text        NOT NULL,
    enabled      boolean     NOT NULL DEFAULT true,
    created_at   timestamptz NOT NULL DEFAULT now()
);

-- Send log and job queue in one: workers claim queued rows with FOR UPDATE SKIP LOCKED.
-- dedupe_key (e.g. 'invoice:<id>:upcoming:7') makes automated sends idempotent.
CREATE TABLE reminder_sends (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id           uuid        REFERENCES reminder_rules (id) ON DELETE SET NULL,
    invoice_id        uuid        REFERENCES invoices (id) ON DELETE SET NULL,
    kind              text        NOT NULL CHECK (kind IN ('upcoming', 'overdue', 'budget')),
    mode              text        NOT NULL CHECK (mode IN ('manual', 'automated')),
    recipient         text        NOT NULL,
    subject           text        NOT NULL,
    body              text        NOT NULL,
    status            text        NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'sent', 'failed')),
    scheduled_for     timestamptz NOT NULL DEFAULT now(),
    sent_at           timestamptz,
    error             text,
    gmail_message_id  text,
    dedupe_key        text        UNIQUE,
    created_by        uuid        REFERENCES users (id),
    created_at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX reminder_sends_queue_idx ON reminder_sends (scheduled_for) WHERE status = 'queued';
CREATE INDEX reminder_sends_invoice_idx ON reminder_sends (invoice_id);

CREATE TABLE financing_requests (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id    uuid        NOT NULL REFERENCES invoices (id),
    amount_cents  bigint      NOT NULL CHECK (amount_cents > 0),
    status        text        NOT NULL DEFAULT 'requested' CHECK (status IN ('requested', 'approved', 'declined')),
    created_by    uuid        NOT NULL REFERENCES users (id),
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE audit_log (
    id         bigserial PRIMARY KEY,
    at         timestamptz NOT NULL DEFAULT now(),
    user_id    uuid        REFERENCES users (id) ON DELETE SET NULL,
    action     text        NOT NULL,
    entity     text        NOT NULL,
    entity_id  text,
    details    jsonb
);
CREATE INDEX audit_log_at_idx ON audit_log (at DESC);
