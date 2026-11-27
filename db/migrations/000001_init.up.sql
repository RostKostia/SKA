-- Kerdos schema, version 1.
-- Merges the first db/ schema (single company, Google sign-in, Gmail queue, payable and
-- receivable invoices, budgets, audit log) with docs/data-model.md and the team decisions of
-- 2026-11-27: automatic invoicing, reminders timed from the due date with escalating tone,
-- unpaid invoices in the forecast on each client's usual paying day, no reminders for
-- financed invoices.
--
-- Conventions: money is integer cents (bigint); business dates are DATE; instants are
-- timestamptz; enums are text + CHECK; ids are uuid. The company table holds exactly one row.

BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ---------------------------------------------------------------------------
-- Company, users, sign-in
-- ---------------------------------------------------------------------------

CREATE TABLE company (
    id                  boolean PRIMARY KEY DEFAULT true CHECK (id),
    name                text         NOT NULL,
    legal_name          text,
    tax_id              text,
    email               text,
    phone               text,
    address             text,
    iban                text,
    currency            char(3)      NOT NULL DEFAULT 'EUR',
    vat_registered      boolean      NOT NULL DEFAULT true,
    vat_rate_percent    numeric(4,2) NOT NULL DEFAULT 19.00 CHECK (vat_rate_percent BETWEEN 0 AND 100),
    payment_terms_days  integer      NOT NULL DEFAULT 14 CHECK (payment_terms_days >= 0),
    invoice_prefix      text         NOT NULL DEFAULT 'KRD',
    language            text         NOT NULL DEFAULT 'en' CHECK (language IN ('en', 'el', 'ru')),
    timezone            text         NOT NULL DEFAULT 'Europe/Nicosia',
    -- Demo only: "Skip time" adds to this. Always 0 in real use. Every "today" comes from kerdos_today().
    clock_offset_days   integer      NOT NULL DEFAULT 0 CHECK (clock_offset_days >= 0)
);

-- The business's "today": the local date in the company's timezone plus the demo offset.
CREATE FUNCTION kerdos_today() RETURNS date
LANGUAGE sql STABLE AS $$
    SELECT (now() AT TIME ZONE c.timezone)::date + c.clock_offset_days FROM company c;
$$;

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

-- ---------------------------------------------------------------------------
-- Reminder policies and templates (before counterparties, which can override the policy)
-- ---------------------------------------------------------------------------

-- When to chase. Overdue reminder 1 goes out first_after_due_days after the due date; each later
-- one every_days after the last one actually sent (automatic or manual). Tone n is
-- tone_steps[min(n, length)]. before_due_days adds friendly nudges before the due date.
CREATE TABLE reminder_policies (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name                  text        NOT NULL UNIQUE,
    is_default            boolean     NOT NULL DEFAULT false,
    before_due_days       integer[]   NOT NULL DEFAULT '{}' CHECK (0 < ALL (before_due_days)),
    first_after_due_days  integer     NOT NULL DEFAULT 3 CHECK (first_after_due_days >= 0),
    every_days            integer     NOT NULL DEFAULT 3 CHECK (every_days > 0),
    tone_steps            text[]      NOT NULL DEFAULT '{polite,firm,final}'
                          CHECK (cardinality(tone_steps) > 0 AND tone_steps <@ '{polite,firm,final}'::text[]),
    max_reminders         integer     CHECK (max_reminders > 0), -- NULL: keep sending the last tone
    enabled               boolean     NOT NULL DEFAULT true,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX reminder_policies_one_default ON reminder_policies (is_default) WHERE is_default;

-- What to say. Placeholders: {company_name}, {client_name}, {invoice_number}, {amount},
-- {due_date}, {days_overdue}, {payment_link}; the owner summary uses {summary}; budget alerts
-- use {category}, {forecast}, {budget}.
CREATE TABLE message_templates (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    kind        text        NOT NULL CHECK (kind IN ('invoice', 'upcoming', 'overdue', 'receipt', 'owner_summary', 'budget')),
    tone        text        CHECK (tone IN ('polite', 'firm', 'final')),
    language    text        NOT NULL DEFAULT 'en' CHECK (language IN ('en', 'el', 'ru')),
    subject     text        NOT NULL,
    body        text        NOT NULL,
    enabled     boolean     NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CHECK ((kind = 'overdue') = (tone IS NOT NULL))
);
CREATE UNIQUE INDEX message_templates_key ON message_templates (kind, coalesce(tone, ''), language);

-- ---------------------------------------------------------------------------
-- Counterparties (clients and suppliers)
-- ---------------------------------------------------------------------------

CREATE TABLE counterparties (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name                text        NOT NULL,
    aliases             text[]      NOT NULL DEFAULT '{}', -- other names the chat should match: 'Kostas', 'Κώστας'
    email               text,                              -- required before an invoice can be sent
    phone               text,
    tax_id              text,
    language            text        CHECK (language IN ('en', 'el', 'ru')), -- NULL: company language
    jinius_party_id     text,
    reminder_policy_id  uuid        REFERENCES reminder_policies (id) ON DELETE SET NULL, -- NULL: default policy
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX counterparties_name_key ON counterparties (lower(name));
CREATE INDEX counterparties_aliases_idx ON counterparties USING gin (aliases);

-- ---------------------------------------------------------------------------
-- Invoices
-- ---------------------------------------------------------------------------

CREATE TABLE invoice_counters (
    year        smallint PRIMARY KEY,
    last_value  integer NOT NULL
);

-- Next receivable invoice number, e.g. KRD-2026-0001. Atomic per year; numbers are never reused.
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

CREATE TABLE financing_offers (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    forecast_run_id  uuid,        -- FK added below, after forecast_runs
    product          text        NOT NULL CHECK (product IN ('invoice_financing', 'overdraft')),
    amount_cents     bigint      NOT NULL CHECK (amount_cents > 0),
    fee_cents        bigint      NOT NULL DEFAULT 0 CHECK (fee_cents >= 0),
    repayment_date   date,
    status           text        NOT NULL DEFAULT 'offered'
                     CHECK (status IN ('offered', 'accepted', 'declined', 'expired', 'disbursed', 'repaid')),
    offered_at       timestamptz NOT NULL DEFAULT now(),
    decided_at       timestamptz,
    disbursed_at     timestamptz,
    decided_by       uuid        REFERENCES users (id),
    provider         text        NOT NULL DEFAULT 'mock' CHECK (provider IN ('mock', 'boc')),
    external_id      text,
    CHECK ((status = 'offered') = (decided_at IS NULL))
);

-- Status 'overdue' is not stored: it is derived from kerdos_today(), so "Skip time" works.
-- See invoice_overview for the display status.
CREATE TABLE invoices (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    number              text        NOT NULL,
    direction           text        NOT NULL CHECK (direction IN ('payable', 'receivable')),
    counterparty_id     uuid        NOT NULL REFERENCES counterparties (id),
    description         text,
    amount_cents        bigint      NOT NULL CHECK (amount_cents > 0),
    vat_cents           bigint      NOT NULL DEFAULT 0 CHECK (vat_cents >= 0 AND vat_cents <= amount_cents),
    currency            char(3)     NOT NULL DEFAULT 'EUR',
    issue_date          date        NOT NULL,
    due_date            date        NOT NULL,
    -- Chat invoices are sent at once: 'draft' only lasts while the send is in flight or after it failed.
    status              text        NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'sent', 'paid', 'cancelled')),
    paid_at             timestamptz,
    source              text        NOT NULL DEFAULT 'manual' CHECK (source IN ('manual', 'chat', 'import')),
    source_text         text,       -- the chat command, e.g. 'Invoice Kostas, 3 hours at €40'
    provider            text        CHECK (provider IN ('jinius', 'mock')), -- who delivered it (receivables)
    external_id         text,       -- the provider's id (Jinius e-invoice id, or the supplier's reference)
    payment_link_url    text,
    pay_token           text        UNIQUE DEFAULT encode(gen_random_bytes(16), 'hex'), -- public page /pay/{token}
    sent_at             timestamptz,
    financed_by_offer_id uuid       REFERENCES financing_offers (id), -- set when an offer it backs is accepted
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    CHECK (due_date >= issue_date),
    CHECK ((status = 'paid') = (paid_at IS NOT NULL)),
    CHECK (source <> 'chat' OR source_text IS NOT NULL),
    CHECK (direction = 'receivable' OR financed_by_offer_id IS NULL),
    UNIQUE (direction, counterparty_id, number)
);
COMMENT ON COLUMN invoices.amount_cents IS 'Total including VAT';
COMMENT ON COLUMN invoices.vat_cents IS 'VAT portion contained in amount_cents';
CREATE UNIQUE INDEX invoices_receivable_number_key ON invoices (number) WHERE direction = 'receivable';
CREATE UNIQUE INDEX invoices_external_id_key ON invoices (provider, external_id) WHERE external_id IS NOT NULL AND provider IS NOT NULL;
CREATE INDEX invoices_list_idx ON invoices (direction, status, due_date);
CREATE INDEX invoices_counterparty_idx ON invoices (counterparty_id);

-- '3 hours at €40' is one line: quantity 3, unit 'hour', unit price 4000.
CREATE TABLE invoice_lines (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id        uuid          NOT NULL REFERENCES invoices (id) ON DELETE CASCADE,
    position          smallint      NOT NULL,
    description       text          NOT NULL,
    quantity          numeric(10,3) NOT NULL CHECK (quantity > 0),
    unit              text          NOT NULL DEFAULT 'service' CHECK (unit IN ('hour', 'day', 'item', 'service')),
    unit_price_cents  bigint        NOT NULL CHECK (unit_price_cents >= 0),
    vat_rate_percent  numeric(4,2)  NOT NULL DEFAULT 0 CHECK (vat_rate_percent BETWEEN 0 AND 100),
    net_cents         bigint        GENERATED ALWAYS AS (round(quantity * unit_price_cents)::bigint) STORED,
    vat_cents         bigint        GENERATED ALWAYS AS (round(round(quantity * unit_price_cents) * vat_rate_percent / 100)::bigint) STORED,
    UNIQUE (invoice_id, position)
);

-- An invoice with lines must add up: amount = sum(net + VAT), vat = sum(VAT). Checked at commit,
-- so the invoice and its lines can be written in any order inside one transaction.
CREATE FUNCTION check_invoice_totals() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    v_id    uuid;
    v_inv   invoices%ROWTYPE;
    v_net   bigint;
    v_vat   bigint;
    v_lines integer;
BEGIN
    IF TG_TABLE_NAME = 'invoices' THEN
        v_id := NEW.id;
    ELSIF TG_OP = 'DELETE' THEN
        v_id := OLD.invoice_id;
    ELSE
        v_id := NEW.invoice_id;
    END IF;
    SELECT * INTO v_inv FROM invoices WHERE id = v_id;
    IF NOT FOUND THEN RETURN NULL; END IF;
    SELECT count(*), coalesce(sum(net_cents), 0), coalesce(sum(vat_cents), 0)
      INTO v_lines, v_net, v_vat FROM invoice_lines WHERE invoice_id = v_id;
    IF v_lines > 0 AND (v_inv.amount_cents <> v_net + v_vat OR v_inv.vat_cents <> v_vat) THEN
        RAISE EXCEPTION 'invoice % totals (amount %, vat %) do not match its lines (net %, vat %)',
            v_inv.number, v_inv.amount_cents, v_inv.vat_cents, v_net, v_vat
            USING ERRCODE = 'check_violation';
    END IF;
    RETURN NULL;
END $$;

CREATE CONSTRAINT TRIGGER invoices_totals_check
    AFTER INSERT OR UPDATE OF amount_cents, vat_cents ON invoices
    DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_invoice_totals();
CREATE CONSTRAINT TRIGGER invoice_lines_totals_check
    AFTER INSERT OR UPDATE OR DELETE ON invoice_lines
    DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_invoice_totals();

-- Which unpaid invoices back an invoice_financing offer (picked newest first until they cover it).
CREATE TABLE financing_offer_invoices (
    financing_offer_id  uuid   NOT NULL REFERENCES financing_offers (id) ON DELETE CASCADE,
    invoice_id          uuid   NOT NULL REFERENCES invoices (id),
    outstanding_cents   bigint NOT NULL CHECK (outstanding_cents > 0),
    PRIMARY KEY (financing_offer_id, invoice_id)
);

-- ---------------------------------------------------------------------------
-- Bank (BoC) and money movements
-- ---------------------------------------------------------------------------

-- One BoC subscription = the owner's consent. Status maps BoC PENDING -> pending, ACTV -> active.
-- The app token (client credentials) is not stored: it is per app and lives 60 minutes.
CREATE TABLE bank_connections (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    provider         text        NOT NULL CHECK (provider IN ('boc', 'mock')),
    subscription_id  text        NOT NULL,
    status           text        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'active', 'expired', 'revoked')),
    permissions      jsonb       NOT NULL, -- the exact accounts/payments objects sent to POST /v1/subscriptions
    starts_on        date,
    expires_on       date,       -- 180 days after consent
    last_synced_at   timestamptz,
    created_by       uuid        REFERENCES users (id),
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider, subscription_id)
);

CREATE TABLE bank_accounts (
    id                     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    bank_connection_id     uuid        REFERENCES bank_connections (id) ON DELETE SET NULL,
    boc_account_id         text        NOT NULL UNIQUE, -- BoC accountId, e.g. 351012345671 ('sample-…' for mock)
    bank_id                text,
    iban                   text,
    account_type           text,       -- BoC accountType, e.g. CURRENT
    name                   text,       -- BoC accountName or accountAlias
    currency               char(3)     NOT NULL DEFAULT 'EUR',
    is_primary             boolean     NOT NULL DEFAULT false, -- the account the forecast uses
    overdraft_limit_cents  bigint      CHECK (overdraft_limit_cents >= 0),
    created_at             timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX bank_accounts_one_primary ON bank_accounts (is_primary) WHERE is_primary;

-- Each balance fetch (BoC AVAILABLE and CURRENT), so the forecast knows what it started from.
CREATE TABLE balance_snapshots (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    bank_account_id  uuid        NOT NULL REFERENCES bank_accounts (id) ON DELETE CASCADE,
    available_cents  bigint      NOT NULL,
    current_cents    bigint      NOT NULL,
    as_of            timestamptz NOT NULL DEFAULT clock_timestamp(), -- when it was fetched
    business_date    date        NOT NULL DEFAULT kerdos_today(), -- which business day it describes (demo clock)
    source           text        NOT NULL DEFAULT 'boc' CHECK (source IN ('boc', 'mock', 'simulated'))
);
CREATE INDEX balance_snapshots_latest_idx ON balance_snapshots (bank_account_id, business_date DESC, as_of DESC);

-- Known money in or out that is not an invoice: salaries, the 13th salary, VAT, a deposit.
-- (Rent and insurance bills can also be payable invoices; use one or the other for the same bill.)
CREATE TABLE cashflows (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    direction     text        NOT NULL CHECK (direction IN ('in', 'out')),
    label         text        NOT NULL,
    category      text,
    amount_cents  bigint      NOT NULL CHECK (amount_cents > 0),
    next_date     date        NOT NULL,
    recurrence    text        NOT NULL DEFAULT 'once' CHECK (recurrence IN ('once', 'weekly', 'monthly', 'yearly')),
    end_date      date,
    source        text        NOT NULL DEFAULT 'manual' CHECK (source IN ('manual', 'detected', 'demo')),
    active        boolean     NOT NULL DEFAULT true,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    CHECK (end_date IS NULL OR end_date >= next_date)
);

-- BoC statement rows. BoC sends unsigned euro amounts plus dcInd (DEBIT/CREDIT), which is
-- missing on some sandbox rows: then dc_ind is NULL, amount_cents is positive, and the row is
-- left out of matching and cash-flow detection.
CREATE TABLE bank_transactions (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    bank_account_id      uuid        NOT NULL REFERENCES bank_accounts (id),
    boc_transaction_id   text        NOT NULL UNIQUE,
    booked_date          date        NOT NULL, -- BoC postingDate
    value_date           date,
    dc_ind               text        CHECK (dc_ind IN ('debit', 'credit')),
    amount_cents         bigint      NOT NULL, -- negative = money out
    currency             char(3)     NOT NULL DEFAULT 'EUR',
    transaction_type     text,
    description          text,
    counterparty_name    text,       -- not in BoC statements; filled from mock/Jinius data
    counterparty_iban    text,
    category             text,
    cashflow_id          uuid        REFERENCES cashflows (id) ON DELETE SET NULL,
    raw                  jsonb,
    imported_at          timestamptz NOT NULL DEFAULT now(),
    CHECK (dc_ind IS NULL OR (dc_ind = 'debit') = (amount_cents < 0)),
    CHECK (dc_ind IS NOT NULL OR amount_cents > 0)
);
CREATE INDEX bank_transactions_booked_idx ON bank_transactions (booked_date);
CREATE INDEX bank_transactions_category_idx ON bank_transactions (category, booked_date);

-- Money received for (or paid against) an invoice. Partial payments are allowed.
CREATE TABLE payments (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id           uuid        NOT NULL REFERENCES invoices (id),
    amount_cents         bigint      NOT NULL CHECK (amount_cents > 0),
    received_on          date        NOT NULL DEFAULT kerdos_today(), -- business date (demo clock)
    method               text        NOT NULL CHECK (method IN ('payment_link', 'pay_by_bank', 'bank_transfer', 'cash', 'other')),
    bank_transaction_id  uuid        UNIQUE REFERENCES bank_transactions (id),
    provider             text        CHECK (provider IN ('jinius', 'boc', 'mock')),
    external_id          text,
    created_at           timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX payments_invoice_idx ON payments (invoice_id);
CREATE UNIQUE INDEX payments_external_id_key ON payments (provider, external_id) WHERE external_id IS NOT NULL AND provider IS NOT NULL;

-- "Pay now" on the client's page through BoC payment initiation (pay by bank).
CREATE TABLE payment_initiations (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id       uuid        NOT NULL REFERENCES invoices (id),
    provider         text        NOT NULL CHECK (provider IN ('boc', 'mock')),
    payment_id       text        UNIQUE,  -- BoC paymentId
    creditor_iban    text        NOT NULL,
    amount_cents     bigint      NOT NULL CHECK (amount_cents > 0),
    payment_details  text        NOT NULL, -- starts with the invoice number, so the statement matches exactly
    charges          jsonb,
    status           text        NOT NULL DEFAULT 'initiated'
                     CHECK (status IN ('initiated', 'awaiting_client', 'executed', 'failed', 'abandoned')),
    ref_number       text,       -- BoC refNumber
    error            text,
    resulting_payment_id uuid      UNIQUE REFERENCES payments (id), -- the Payment created on 'executed'
    created_at       timestamptz NOT NULL DEFAULT now(),
    executed_at      timestamptz,
    CHECK ((status = 'executed') = (executed_at IS NOT NULL))
);

-- When an invoice is paid in full: mark it paid and cancel its queued reminders.
CREATE FUNCTION settle_invoice_on_payment() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    v_inv  invoices%ROWTYPE;
    v_paid bigint;
BEGIN
    SELECT * INTO v_inv FROM invoices WHERE id = NEW.invoice_id FOR UPDATE;
    IF v_inv.status = 'cancelled' THEN
        RAISE EXCEPTION 'invoice % is cancelled', v_inv.number USING ERRCODE = 'check_violation';
    END IF;
    SELECT sum(amount_cents) INTO v_paid FROM payments WHERE invoice_id = NEW.invoice_id;
    IF v_paid >= v_inv.amount_cents AND v_inv.status <> 'paid' THEN
        UPDATE invoices SET status = 'paid', paid_at = now(), updated_at = now() WHERE id = v_inv.id;
        UPDATE outbound_messages SET status = 'cancelled'
         WHERE invoice_id = v_inv.id AND status = 'queued' AND kind IN ('upcoming', 'overdue');
    END IF;
    RETURN NULL;
END $$;

-- ---------------------------------------------------------------------------
-- Outgoing messages: one send log and job queue for every email Kerdos sends
-- (invoice delivery, nudges before the due date, overdue reminders, receipts, owner summary,
-- budget alerts). Workers claim queued rows with FOR UPDATE SKIP LOCKED.
-- ---------------------------------------------------------------------------

CREATE TABLE outbound_messages (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    kind                 text        NOT NULL CHECK (kind IN ('invoice', 'upcoming', 'overdue', 'receipt', 'owner_summary', 'budget')),
    mode                 text        NOT NULL CHECK (mode IN ('manual', 'automated')),
    invoice_id           uuid        REFERENCES invoices (id) ON DELETE SET NULL,
    sequence             integer     CHECK (sequence > 0),  -- overdue reminder number: 1, 2, 3…
    tone                 text        CHECK (tone IN ('polite', 'firm', 'final')),
    policy_id            uuid        REFERENCES reminder_policies (id) ON DELETE SET NULL,
    template_id          uuid        REFERENCES message_templates (id) ON DELETE SET NULL,
    channel              text        NOT NULL DEFAULT 'email' CHECK (channel IN ('email')),
    recipient            text        NOT NULL,
    subject              text        NOT NULL,
    body                 text        NOT NULL,
    language             text        NOT NULL DEFAULT 'en' CHECK (language IN ('en', 'el', 'ru')),
    status               text        NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'sent', 'failed', 'cancelled')),
    business_date        date        NOT NULL DEFAULT kerdos_today(), -- the business day it belongs to (demo clock)
    scheduled_for        timestamptz NOT NULL DEFAULT now(),
    sent_at              timestamptz,
    error                text,
    provider             text        CHECK (provider IN ('gmail', 'smtp', 'mock')),
    provider_message_id  text,       -- e.g. the Gmail message id
    dedupe_key           text        UNIQUE, -- e.g. 'invoice:<id>:overdue:2' makes automated sends idempotent
    created_by           uuid        REFERENCES users (id),
    created_at           timestamptz NOT NULL DEFAULT now(),
    CHECK ((kind = 'overdue') = (sequence IS NOT NULL AND tone IS NOT NULL)),
    CHECK (kind IN ('owner_summary', 'budget') OR invoice_id IS NOT NULL),
    CHECK ((status = 'sent') = (sent_at IS NOT NULL))
);
CREATE INDEX outbound_messages_queue_idx ON outbound_messages (scheduled_for) WHERE status = 'queued';
CREATE INDEX outbound_messages_invoice_idx ON outbound_messages (invoice_id);
-- One live overdue reminder per number per invoice.
CREATE UNIQUE INDEX outbound_messages_overdue_seq_key ON outbound_messages (invoice_id, sequence)
    WHERE kind = 'overdue' AND status IN ('queued', 'sent');

CREATE TRIGGER payments_settle_invoice
    AFTER INSERT ON payments FOR EACH ROW EXECUTE FUNCTION settle_invoice_on_payment();

-- ---------------------------------------------------------------------------
-- Budgets, forecast runs, chat, audit
-- ---------------------------------------------------------------------------

CREATE TABLE budgets (
    month         date    NOT NULL CHECK (extract(day FROM month) = 1),
    category      text    NOT NULL,
    amount_cents  bigint  NOT NULL CHECK (amount_cents >= 0),
    PRIMARY KEY (month, category)
);

-- A stored forecast, written whenever Kerdos shows a warning, so an offer can point to its numbers.
CREATE TABLE forecast_runs (
    id                       uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    as_of_date               date        NOT NULL,
    horizon_days             integer     NOT NULL CHECK (horizon_days > 0),
    start_balance_cents      bigint      NOT NULL,
    include_unpaid_invoices  boolean     NOT NULL DEFAULT true,
    points                   jsonb       NOT NULL, -- [{"date": "2026-11-28", "balance_cents": 100000}, …]
    first_negative_date      date,
    lowest_date              date,
    lowest_balance_cents     bigint      NOT NULL,
    shortfall_cents          bigint      NOT NULL CHECK (shortfall_cents >= 0),
    created_at               timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE financing_offers
    ADD CONSTRAINT financing_offers_forecast_run_id_fkey FOREIGN KEY (forecast_run_id) REFERENCES forecast_runs (id);

-- The running conversation with Kerdos. Cards reference rows by id, so they always show current status.
CREATE TABLE chat_messages (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid        REFERENCES users (id) ON DELETE SET NULL,
    role        text        NOT NULL CHECK (role IN ('owner', 'assistant', 'system')),
    text        text        NOT NULL,
    intent      text        CHECK (intent IN ('create_invoice', 'provide_details', 'remind', 'forecast', 'financing', 'debtors', 'question', 'unknown')),
    parsed      jsonb,      -- Claude's structured reading of an owner message
    card        jsonb,      -- e.g. {"type": "invoice", "invoice_id": "…"}
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX chat_messages_created_idx ON chat_messages (created_at);

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

-- Keep updated_at current.
CREATE FUNCTION set_updated_at() RETURNS trigger
LANGUAGE plpgsql AS $$ BEGIN NEW.updated_at := now(); RETURN NEW; END $$;

CREATE TRIGGER reminder_policies_updated_at BEFORE UPDATE ON reminder_policies FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER message_templates_updated_at BEFORE UPDATE ON message_templates FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER counterparties_updated_at    BEFORE UPDATE ON counterparties    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER invoices_updated_at          BEFORE UPDATE ON invoices          FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER bank_connections_updated_at  BEFORE UPDATE ON bank_connections  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER cashflows_updated_at         BEFORE UPDATE ON cashflows         FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- Derived views and functions
-- ---------------------------------------------------------------------------

-- How late each client usually pays: paid date (last payment) minus due date, over paid receivables.
CREATE VIEW counterparty_payment_stats AS
SELECT i.counterparty_id,
       count(*)                                              AS paid_invoices,
       round(avg(p.last_received_on - i.due_date))::integer   AS avg_days_late  -- negative = early
FROM invoices i
JOIN LATERAL (SELECT max(received_on) AS last_received_on FROM payments WHERE invoice_id = i.id) p ON true
WHERE i.direction = 'receivable' AND i.status = 'paid' AND p.last_received_on IS NOT NULL
GROUP BY i.counterparty_id;

-- Every invoice with what is still owed and the status to show.
CREATE VIEW invoice_overview AS
SELECT i.*,
       c.name                                         AS counterparty_name,
       coalesce(p.paid_cents, 0)::bigint              AS paid_cents,
       (i.amount_cents - coalesce(p.paid_cents, 0))::bigint AS outstanding_cents,
       (fo.id IS NOT NULL)                            AS is_financed,
       CASE
           WHEN i.status IN ('draft', 'cancelled')               THEN i.status
           WHEN i.status = 'paid'                                 THEN 'paid'
           WHEN fo.id IS NOT NULL                                 THEN 'financed'
           WHEN kerdos_today() > i.due_date                       THEN 'overdue'
           WHEN coalesce(p.paid_cents, 0) > 0                     THEN 'partially_paid'
           ELSE 'unpaid'
       END                                            AS display_status,
       greatest(kerdos_today() - i.due_date, 0)       AS days_overdue
FROM invoices i
JOIN counterparties c ON c.id = i.counterparty_id
LEFT JOIN (SELECT invoice_id, sum(amount_cents)::bigint AS paid_cents FROM payments GROUP BY invoice_id) p ON p.invoice_id = i.id
LEFT JOIN financing_offers fo ON fo.id = i.financed_by_offer_id AND fo.status IN ('accepted', 'disbursed', 'repaid');

-- Who owes what: open, non-financed receivables per client.
CREATE VIEW debtors AS
SELECT o.counterparty_id,
       o.counterparty_name,
       count(*)                                    AS open_invoices,
       sum(o.outstanding_cents)::bigint            AS outstanding_cents,
       max(kerdos_today() - o.issue_date)          AS oldest_age_days,
       max(o.days_overdue)                         AS max_days_overdue,
       (SELECT count(*) FROM outbound_messages m
         WHERE m.invoice_id = ANY (array_agg(o.id)) AND m.kind = 'overdue' AND m.status = 'sent') AS reminders_sent,
       (SELECT m.tone FROM outbound_messages m
         WHERE m.invoice_id = ANY (array_agg(o.id)) AND m.kind = 'overdue' AND m.status = 'sent'
         ORDER BY m.business_date DESC, m.created_at DESC LIMIT 1)                               AS last_tone
FROM invoice_overview o
WHERE o.direction = 'receivable' AND o.status = 'sent' AND NOT o.is_financed AND o.outstanding_cents > 0
GROUP BY o.counterparty_id, o.counterparty_name;

-- Reminders that should go out today (or are late). The worker (or "Skip time", one day at a time)
-- turns each row into an outbound_messages row with the given dedupe_key.
CREATE VIEW reminders_due AS
WITH base AS (
    SELECT o.id AS invoice_id, o.number, o.counterparty_id, o.due_date, o.outstanding_cents,
           rp.id AS policy_id, rp.before_due_days, rp.first_after_due_days, rp.every_days,
           rp.tone_steps, rp.max_reminders
    FROM invoice_overview o
    JOIN counterparties c ON c.id = o.counterparty_id
    JOIN reminder_policies rp ON rp.id = coalesce(c.reminder_policy_id, (SELECT id FROM reminder_policies WHERE is_default))
    WHERE o.direction = 'receivable' AND o.status = 'sent' AND NOT o.is_financed
      AND o.outstanding_cents > 0 AND rp.enabled
),
overdue AS (
    SELECT b.*,
           coalesce(s.sent_count, 0) + 1 AS next_sequence,
           CASE WHEN s.last_date IS NULL THEN b.due_date + b.first_after_due_days
                ELSE s.last_date + b.every_days END AS due_on
    FROM base b
    LEFT JOIN LATERAL (
        SELECT count(*) AS sent_count, max(business_date) AS last_date
        FROM outbound_messages m
        WHERE m.invoice_id = b.invoice_id AND m.kind = 'overdue' AND m.status IN ('queued', 'sent')
    ) s ON true
)
SELECT 'overdue'::text AS kind, invoice_id, number, policy_id, next_sequence AS sequence,
       tone_steps[least(next_sequence, cardinality(tone_steps))] AS tone, due_on, outstanding_cents,
       'invoice:' || invoice_id || ':overdue:' || next_sequence AS dedupe_key
FROM overdue
WHERE due_on <= kerdos_today()
  AND (max_reminders IS NULL OR next_sequence <= max_reminders)
UNION ALL
SELECT 'upcoming', b.invoice_id, b.number, b.policy_id, NULL, NULL, b.due_date - d.days, b.outstanding_cents,
       'invoice:' || b.invoice_id || ':upcoming:' || d.days
FROM base b
CROSS JOIN LATERAL unnest(b.before_due_days) AS d(days)
WHERE b.due_date - d.days <= kerdos_today() AND kerdos_today() < b.due_date
  AND NOT EXISTS (SELECT 1 FROM outbound_messages m
                  WHERE m.dedupe_key = 'invoice:' || b.invoice_id || ':upcoming:' || d.days);

-- Money expected in and out over the next p_horizon_days, from kerdos_today():
--  * unpaid, non-financed receivables on max(due + client's usual delay (never earlier), tomorrow);
--  * unpaid payables on max(due, tomorrow);
--  * cash-flow occurrences after today (one due today counts until a bank transaction is linked to it).
CREATE FUNCTION forecast_events(p_horizon_days integer DEFAULT 30)
RETURNS TABLE (on_date date, day integer, amount_cents bigint, kind text, label text, invoice_id uuid, cashflow_id uuid)
LANGUAGE sql STABLE
-- JIT compilation costs ~0.8 s here for a query that runs in milliseconds.
SET jit = off
AS $$
    WITH t AS (SELECT kerdos_today() AS d0),
    ev AS (
        SELECT greatest(o.due_date + greatest(coalesce(s.avg_days_late, 0), 0), t.d0 + 1) AS on_date,
               o.outstanding_cents AS amount_cents, 'receivable'::text AS kind,
               o.counterparty_name || ' · ' || o.number AS label, o.id AS invoice_id, NULL::uuid AS cashflow_id
        FROM invoice_overview o CROSS JOIN t
        LEFT JOIN counterparty_payment_stats s ON s.counterparty_id = o.counterparty_id
        WHERE o.direction = 'receivable' AND o.status = 'sent' AND NOT o.is_financed AND o.outstanding_cents > 0
        UNION ALL
        SELECT greatest(o.due_date, t.d0 + 1), -o.outstanding_cents, 'payable',
               o.counterparty_name || ' · ' || coalesce(o.description, o.number), o.id, NULL
        FROM invoice_overview o CROSS JOIN t
        WHERE o.direction = 'payable' AND o.status = 'sent' AND o.outstanding_cents > 0
        UNION ALL
        SELECT occ::date, CASE c.direction WHEN 'in' THEN c.amount_cents ELSE -c.amount_cents END,
               'cashflow', c.label, NULL, c.id
        FROM cashflows c CROSS JOIN t
        CROSS JOIN LATERAL generate_series(
            c.next_date::timestamp,
            least(coalesce(c.end_date, t.d0 + p_horizon_days), t.d0 + p_horizon_days)::timestamp,
            CASE c.recurrence WHEN 'weekly' THEN interval '1 week' WHEN 'monthly' THEN interval '1 month'
                              WHEN 'yearly' THEN interval '1 year' ELSE interval '100 years' END) AS occ
        WHERE c.active
          AND (occ::date > t.d0
               OR (occ::date = t.d0 AND NOT EXISTS (SELECT 1 FROM bank_transactions bt
                                                    WHERE bt.cashflow_id = c.id AND bt.booked_date = t.d0)))
    )
    SELECT ev.on_date, ev.on_date - t.d0, ev.amount_cents, ev.kind, ev.label, ev.invoice_id, ev.cashflow_id
    FROM ev CROSS JOIN t
    WHERE ev.on_date <= t.d0 + p_horizon_days
    ORDER BY ev.on_date, ev.kind, ev.label;
$$;

-- The latest balance of the primary account.
CREATE VIEW current_balance AS
SELECT DISTINCT ON (a.id) a.id AS bank_account_id, s.available_cents, s.current_cents, s.business_date, s.as_of
FROM bank_accounts a
JOIN balance_snapshots s ON s.bank_account_id = a.id
WHERE a.is_primary
ORDER BY a.id, s.business_date DESC, s.as_of DESC;

-- Projected end-of-day balance for day 0 (today) to p_horizon_days.
CREATE FUNCTION forecast_points(p_horizon_days integer DEFAULT 30)
RETURNS TABLE (day integer, on_date date, balance_cents bigint)
LANGUAGE sql STABLE
-- JIT compilation costs ~0.8 s here for a query that runs in milliseconds.
SET jit = off
AS $$
    SELECT g.day, kerdos_today() + g.day,
           ((SELECT available_cents FROM current_balance)
            + sum(coalesce(e.total, 0)) OVER (ORDER BY g.day))::bigint
    FROM generate_series(0, p_horizon_days) AS g(day)
    LEFT JOIN (SELECT f.day, sum(f.amount_cents) AS total FROM forecast_events(p_horizon_days) f GROUP BY f.day) e
           ON e.day = g.day
    ORDER BY g.day;
$$;

-- "In <lowest_day> days you'll be <shortfall> negative." No row when the balance stays at or above zero.
CREATE FUNCTION forecast_shortfall(p_horizon_days integer DEFAULT 30)
RETURNS TABLE (first_negative_day integer, lowest_day integer, lowest_balance_cents bigint, shortfall_cents bigint)
LANGUAGE sql STABLE
-- JIT compilation costs ~0.8 s here for a query that runs in milliseconds.
SET jit = off
AS $$
    WITH p AS (SELECT * FROM forecast_points(p_horizon_days)),
    lo AS (SELECT day, balance_cents FROM p ORDER BY balance_cents, day LIMIT 1)
    SELECT (SELECT min(day) FROM p WHERE balance_cents < 0), lo.day, lo.balance_cents, -lo.balance_cents
    FROM lo
    WHERE lo.balance_cents < 0;
$$;

-- Accept an offer exactly once: marks its invoices financed and cancels their queued reminders.
-- Returns false (and changes nothing) if the offer is no longer 'offered' or an invoice is no
-- longer open, so a double click is harmless.
CREATE FUNCTION accept_financing_offer(p_offer_id uuid, p_user_id uuid DEFAULT NULL) RETURNS boolean
LANGUAGE plpgsql AS $$
DECLARE
    v_product text;
BEGIN
    UPDATE financing_offers SET status = 'accepted', decided_at = now(), decided_by = p_user_id
     WHERE id = p_offer_id AND status = 'offered'
    RETURNING product INTO v_product;
    IF NOT FOUND THEN RETURN false; END IF;

    IF v_product = 'invoice_financing' THEN
        IF EXISTS (SELECT 1 FROM financing_offer_invoices foi
                   JOIN invoice_overview o ON o.id = foi.invoice_id
                   WHERE foi.financing_offer_id = p_offer_id
                     AND (o.status <> 'sent' OR o.is_financed OR o.outstanding_cents <= 0)) THEN
            RAISE EXCEPTION 'offer % includes an invoice that is no longer open', p_offer_id
                USING ERRCODE = 'check_violation';
        END IF;
        UPDATE invoices SET financed_by_offer_id = p_offer_id
         WHERE id IN (SELECT invoice_id FROM financing_offer_invoices WHERE financing_offer_id = p_offer_id);
        UPDATE outbound_messages SET status = 'cancelled'
         WHERE status = 'queued' AND kind IN ('upcoming', 'overdue')
           AND invoice_id IN (SELECT invoice_id FROM financing_offer_invoices WHERE financing_offer_id = p_offer_id);
    END IF;
    RETURN true;
END $$;

COMMIT;
