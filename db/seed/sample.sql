-- Sample data for development, dated relative to today. Run once on a fresh database:
--   psql -v ON_ERROR_STOP=1 --single-transaction -f sample.sql
-- A second run fails on the company row and changes nothing.

INSERT INTO company (name) VALUES ('Nicosia Studio (sample)');

INSERT INTO users (google_sub, email, name, role)
VALUES ('sample-admin', 'owner@example.com', 'Sample Owner', 'admin');

INSERT INTO counterparties (name, email) VALUES
    ('Elena''s Bakery (sample)',     'elena@example.com'),
    ('Andreas Electric (sample)',    'andreas@example.com'),
    ('Cyprus Tiles Ltd (sample)',    'accounts@example.com'),
    ('Kostas Hadjis (sample)',       'kostas@example.com'),
    ('Studio Landlord (sample)',     'landlord@example.com'),
    ('Cyprus Insurance (sample)',    'billing@example.com');

-- Receivable: from the company to its clients.
INSERT INTO invoices (number, direction, counterparty_id, description, amount_cents, vat_cents,
                      issue_date, due_date, status, paid_at, source)
SELECT next_invoice_number(extract(year FROM current_date)::smallint), 'receivable', c.id, v.descr,
       v.amount, round(v.amount * 19.0 / 119)::bigint,
       current_date + v.issue, current_date + v.due, v.status,
       CASE WHEN v.status = 'paid' THEN now() - interval '6 days' END, 'jinius'
FROM (VALUES
    ('Cyprus Tiles Ltd (sample)', 'Brand refresh',        60000, -20,  -6, 'paid'),
    ('Kostas Hadjis (sample)',    'Consulting, 3 hours',  12000, -50,  -5, 'overdue'),
    ('Elena''s Bakery (sample)',  'Menu and logo design', 45000, -10,   4, 'sent'),
    ('Andreas Electric (sample)', 'Website update',       38000,  -5,   9, 'sent')
) AS v(client, descr, amount, issue, due, status)
JOIN counterparties c ON c.name = v.client;

INSERT INTO invoice_lines (invoice_id, position, description, quantity, unit_price_cents)
SELECT id, 1, coalesce(description, number), 1, amount_cents - vat_cents
FROM invoices WHERE direction = 'receivable';

-- Payable: from suppliers to the company.
INSERT INTO invoices (number, direction, counterparty_id, description, amount_cents, vat_cents,
                      issue_date, due_date, status, source)
SELECT v.number, 'payable', c.id, v.descr, v.amount, 0,
       current_date + v.issue, current_date + v.due, 'sent', 'manual'
FROM (VALUES
    ('Studio Landlord (sample)',  'RENT-2026-11', 'Studio rent', 150000, -26,  4),
    ('Cyprus Insurance (sample)', 'INS-88213',    'Insurance',    60000, -18, 12)
) AS v(supplier, number, descr, amount, issue, due)
JOIN counterparties c ON c.name = v.supplier;

INSERT INTO bank_accounts (boc_account_id, iban, balance_cents, balance_at)
VALUES ('sample-account-1', 'CY00000000000000000000000000', 100000, now());

-- Six months of history for the category forecast.
INSERT INTO bank_transactions (bank_account_id, boc_transaction_id, booked_date, amount_cents,
                               description, category)
SELECT (SELECT id FROM bank_accounts LIMIT 1),
       'sample-' || m.n || '-' || t.category,
       (date_trunc('month', current_date) - make_interval(months => m.n))::date + (t.day - 1),
       t.amount, t.description, t.category
FROM generate_series(1, 6) AS m(n)
CROSS JOIN LATERAL (VALUES
    ('Rent',      1, -150000,                          'Studio rent'),
    ('Utilities', 5, -(9000 + (m.n * 1700) % 4000),    'Electricity and internet'),
    ('Software', 10,  -5900,                           'Design software'),
    ('Supplies', 15, -(11000 + (m.n * 2300) % 9000),   'Materials'),
    ('Income',   25,  95000,                           'Client payments')
) AS t(category, day, amount, description);

INSERT INTO bank_transactions (bank_account_id, boc_transaction_id, booked_date, amount_cents,
                               description, category)
SELECT (SELECT id FROM bank_accounts LIMIT 1), 'sample-' || m.n || '-Insurance',
       (date_trunc('month', current_date) - make_interval(months => m.n))::date + 19,
       -60000, 'Insurance', 'Insurance'
FROM (VALUES (3), (6)) AS m(n);

INSERT INTO budgets (month, category, amount_cents)
SELECT date_trunc('month', current_date + interval '1 month')::date, category, amount
FROM (VALUES ('Rent', 150000), ('Utilities', 12000), ('Software', 8000),
             ('Supplies', 15000), ('Insurance', 20000)) AS b(category, amount);

INSERT INTO reminder_rules (kind, offsets_days, subject, body) VALUES
    ('upcoming', '{7,3,1}', 'Invoice {invoice_number} is due on {due_date}',
     'Hi {client_name}, a friendly reminder that invoice {invoice_number} for {amount} is due on {due_date}.'),
    ('overdue',  '{1,7,14}', 'Invoice {invoice_number} is overdue',
     'Hi {client_name}, invoice {invoice_number} for {amount} was due on {due_date} and is now overdue. Please arrange payment.'),
    ('budget',   '{}',       'Budget warning: {category}',
     'Forecast spend for {category} is {forecast}, against a budget of {budget}.');
