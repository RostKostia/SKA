-- Sample data for development and the demo, dated relative to the business's today
-- (kerdos_today()). Run once on a fresh database:
--   psql -v ON_ERROR_STOP=1 --single-transaction -f sample.sql
-- A second run fails on the company row and changes nothing.
--
-- The demo story this sets up (day 0 = today):
--   balance €1,000; +€300 deposit on day 2; rent −€1,500 on day 4; insurance −€600 on day 12.
--   Elena (pays ~45 days late) owes €450 due day 4, expected day 49: outside the 30-day window.
--   Andreas (pays ~30 days late) owes €380, due day −5 (overdue), expected day 25.
--   → "In 12 days you'll be €800 negative." See db/tests/demo_story.sql for the full walk-through.

INSERT INTO company (name, email, iban, vat_registered, payment_terms_days)
VALUES ('Nicosia Studio', 'owner@example.com', 'CY00 0000 0000 0000 0000 0000 0000', false, 14);

INSERT INTO users (google_sub, email, name, role)
VALUES ('sample-admin', 'owner@example.com', 'Studio Owner', 'admin');

-- Reminders: first overdue reminder 3 days after the due date, then every 3 days,
-- polite → firm → final. No nudges before the due date in the demo (add e.g. '{3}' for one).
INSERT INTO reminder_policies (name, is_default, before_due_days, first_after_due_days, every_days)
VALUES ('Default', true, '{}', 3, 3);

INSERT INTO message_templates (kind, tone, subject, body) VALUES
    ('invoice', NULL, 'Invoice {invoice_number} from {company_name}',
     'Hi {client_name}, here is invoice {invoice_number} for {amount}, due on {due_date}. You can pay in one tap: {payment_link}'),
    ('upcoming', NULL, 'Invoice {invoice_number} is due on {due_date}',
     'Hi {client_name}, a friendly reminder that invoice {invoice_number} for {amount} is due on {due_date}. {payment_link}'),
    ('overdue', 'polite', 'Reminder: invoice {invoice_number}',
     'Hi {client_name}, a friendly reminder that invoice {invoice_number} for {amount} is still open. You can pay in one tap: {payment_link}'),
    ('overdue', 'firm', 'Invoice {invoice_number} is {days_overdue} days overdue',
     'Hi {client_name}, invoice {invoice_number} ({amount}) is now {days_overdue} days overdue. Please pay it today: {payment_link}'),
    ('overdue', 'final', 'Final notice: invoice {invoice_number}',
     'Final notice: invoice {invoice_number} ({amount}) is {days_overdue} days overdue. Please pay today, or reply to agree a plan. {payment_link}'),
    ('receipt', NULL, 'Payment received for invoice {invoice_number}',
     'Hi {client_name}, thank you. We received {amount} for invoice {invoice_number}.'),
    ('owner_summary', NULL, 'Who owes you money',
     '{summary}'),
    ('budget', NULL, 'Budget warning: {category}',
     'Forecast spend for {category} is {forecast}, against a budget of {budget}.');

-- Kostas is the client invoiced live in the demo. His email should be the second phone's
-- address: UPDATE counterparties SET email = '<demo phone email>' WHERE name LIKE 'Kostas%';
INSERT INTO counterparties (name, aliases, email) VALUES
    ('Elena''s Bakery',   '{Elena,"Elena''s Bakery"}', 'elena@example.com'),
    ('Andreas Electric',  '{Andreas,"Andreas Electric"}', 'andreas@example.com'),
    ('Cyprus Tiles Ltd',  '{"Cyprus Tiles"}', 'accounts@example.com'),
    ('Kostas Hadjis',     '{Kostas,Κώστας}', 'kostas@example.com'),
    ('Studio Landlord',   '{Landlord}', 'landlord@example.com'),
    ('Cyprus Insurance',  '{}', 'billing@example.com');

-- Receivables, numbered in issue order. The first three are paid history; their payment dates
-- give each client's usual delay (Elena 45 days late, Andreas 30 late, Cyprus Tiles 2 days early).
DO $$
DECLARE
    d0 date := kerdos_today();
    r  record;
    v_id uuid;
BEGIN
    FOR r IN
        SELECT * FROM (VALUES
            ('Elena''s Bakery',   'Bread labels',         30000, -120, -106, -61),
            ('Andreas Electric',  'Logo design',          25000,  -90,  -76, -46),
            ('Cyprus Tiles Ltd',  'Brand refresh',        60000,  -20,   -6,  -8),
            ('Andreas Electric',  'Website update',       38000,  -19,   -5, NULL),
            ('Elena''s Bakery',   'Menu and logo design', 45000,  -10,    4, NULL)
        ) AS v(client, descr, amount, issue, due, paid)
        ORDER BY issue
    LOOP
        INSERT INTO invoices (number, direction, counterparty_id, description, amount_cents, vat_cents,
                              issue_date, due_date, status, source, provider, external_id, sent_at,
                              payment_link_url)
        SELECT next_invoice_number(extract(year FROM d0 + r.issue)::smallint), 'receivable', c.id,
               r.descr, r.amount, 0, d0 + r.issue, d0 + r.due, 'sent', 'manual', 'mock',
               NULL, (d0 + r.issue)::timestamptz + interval '10 hours', NULL
        FROM counterparties c WHERE c.name = r.client
        RETURNING id INTO v_id;

        UPDATE invoices SET external_id = 'mock-' || id, payment_link_url = '/pay/' || pay_token WHERE id = v_id;

        INSERT INTO invoice_lines (invoice_id, position, description, quantity, unit, unit_price_cents)
        VALUES (v_id, 1, r.descr, 1, 'service', r.amount);

        INSERT INTO outbound_messages (kind, mode, invoice_id, recipient, subject, body, status,
                                       business_date, scheduled_for, sent_at, provider, dedupe_key)
        SELECT 'invoice', 'automated', v_id, c.email,
               'Invoice ' || i.number || ' from Nicosia Studio',
               'Hi ' || c.name || ', here is invoice ' || i.number || ' for €' || (r.amount / 100) || '.',
               'sent', i.issue_date, i.sent_at, i.sent_at, 'mock', 'invoice:' || v_id || ':sent'
        FROM invoices i JOIN counterparties c ON c.id = i.counterparty_id WHERE i.id = v_id;

        IF r.paid IS NOT NULL THEN
            INSERT INTO payments (invoice_id, amount_cents, received_on, method, provider, external_id)
            VALUES (v_id, r.amount, d0 + r.paid, 'bank_transfer', 'mock', 'mock-pay-' || v_id);
            -- The payment trigger marks the invoice paid at now(); show the business date instead.
            UPDATE invoices SET paid_at = (d0 + r.paid)::timestamptz + interval '12 hours' WHERE id = v_id;
        END IF;
    END LOOP;
END $$;

-- Andreas's open invoice is overdue: reminder 1 (polite) went out on day −2, so reminder 2 is due on day 1.
INSERT INTO outbound_messages (kind, mode, invoice_id, sequence, tone, policy_id, recipient, subject, body,
                               status, business_date, scheduled_for, sent_at, provider, dedupe_key)
SELECT 'overdue', 'automated', i.id, 1, 'polite', p.id, c.email,
       'Reminder: invoice ' || i.number,
       'Hi ' || c.name || ', a friendly reminder that invoice ' || i.number || ' for €380 is still open.',
       'sent', kerdos_today() - 2, now() - interval '2 days', now() - interval '2 days', 'mock',
       'invoice:' || i.id || ':overdue:1'
FROM invoices i
JOIN counterparties c ON c.id = i.counterparty_id
CROSS JOIN reminder_policies p
WHERE c.name = 'Andreas Electric' AND i.status = 'sent' AND p.is_default;

-- Payables: bills the company owes.
INSERT INTO invoices (number, direction, counterparty_id, description, amount_cents, vat_cents,
                      issue_date, due_date, status, source, external_id)
SELECT v.number, 'payable', c.id, v.descr, v.amount, 0,
       kerdos_today() + v.issue, kerdos_today() + v.due, 'sent', 'manual', NULL
FROM (VALUES
    ('Studio Landlord',  'RENT-' || to_char(kerdos_today() + 4, 'YYYY-MM'), 'Studio rent', 150000, -26,  4),
    ('Cyprus Insurance', 'INS-88213',                                       'Insurance',    60000, -18, 12)
) AS v(supplier, number, descr, amount, issue, due)
JOIN counterparties c ON c.name = v.supplier;

-- Other known money: a client's deposit arriving on day 2.
INSERT INTO cashflows (direction, label, category, amount_cents, next_date, recurrence, source)
VALUES ('in', 'Maria Savva, deposit', 'Income', 30000, kerdos_today() + 2, 'once', 'demo');

-- Bank: a mock BoC connection with one primary account holding €1,000.
INSERT INTO bank_connections (provider, subscription_id, status, permissions, starts_on, expires_on)
VALUES ('mock', 'sample-subscription', 'active',
        '{"accounts": {"transactionHistory": true, "balance": true, "details": true, "checkFundsAvailability": false}}',
        kerdos_today(), kerdos_today() + 180);

INSERT INTO bank_accounts (bank_connection_id, boc_account_id, iban, account_type, name, is_primary)
SELECT id, 'sample-account-1', 'CY00 0000 0000 0000 0000 0000 0000', 'CURRENT', 'Business current account', true
FROM bank_connections WHERE subscription_id = 'sample-subscription';

INSERT INTO balance_snapshots (bank_account_id, available_cents, current_cents, source)
SELECT id, 100000, 100000, 'mock' FROM bank_accounts WHERE boc_account_id = 'sample-account-1';

-- Six months of history for the spending-by-category forecast and budgets.
INSERT INTO bank_transactions (bank_account_id, boc_transaction_id, booked_date, dc_ind, amount_cents,
                               description, category)
SELECT (SELECT id FROM bank_accounts WHERE boc_account_id = 'sample-account-1'),
       'sample-' || m.n || '-' || t.category,
       (date_trunc('month', kerdos_today()) - make_interval(months => m.n))::date + (t.day - 1),
       CASE WHEN t.amount < 0 THEN 'debit' ELSE 'credit' END,
       t.amount, t.description, t.category
FROM generate_series(1, 6) AS m(n)
CROSS JOIN LATERAL (VALUES
    ('Rent',      1, -150000,                          'Studio rent'),
    ('Utilities', 5, -(9000 + (m.n * 1700) % 4000),    'Electricity and internet'),
    ('Software', 10,  -5900,                           'Design software'),
    ('Supplies', 15, -(11000 + (m.n * 2300) % 9000),   'Materials'),
    ('Income',   25,  95000,                           'Client payments')
) AS t(category, day, amount, description);

INSERT INTO bank_transactions (bank_account_id, boc_transaction_id, booked_date, dc_ind, amount_cents,
                               description, category)
SELECT (SELECT id FROM bank_accounts WHERE boc_account_id = 'sample-account-1'), 'sample-' || m.n || '-Insurance',
       (date_trunc('month', kerdos_today()) - make_interval(months => m.n))::date + 19,
       'debit', -60000, 'Insurance', 'Insurance'
FROM (VALUES (3), (6)) AS m(n);

INSERT INTO budgets (month, category, amount_cents)
SELECT date_trunc('month', kerdos_today() + interval '1 month')::date, category, amount
FROM (VALUES ('Rent', 150000), ('Utilities', 12000), ('Software', 8000),
             ('Supplies', 15000), ('Insurance', 20000)) AS b(category, amount);

INSERT INTO chat_messages (role, text)
VALUES ('assistant', 'Good morning. Tell me who to invoice and I''ll send it through Jinius with a payment link.');
