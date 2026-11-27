-- Walks the 2-minute demo against a freshly seeded database and checks the numbers and the
-- safety rules. Everything runs in one transaction that is rolled back, so the database is
-- left exactly as it was.
--
--   psql -v ON_ERROR_STOP=1 -f db/tests/demo_story.sql
--
-- pg_temp.simulate_day() below is what the backend's "Skip time" must do for each skipped day:
-- move the clock, let the (mock) bank post that day's money, and send the reminders due.

\set QUIET on
\o /dev/null
BEGIN;

CREATE FUNCTION pg_temp.check(p_ok boolean, p_what text) RETURNS void
LANGUAGE plpgsql AS $$
BEGIN
    IF p_ok IS NOT TRUE THEN RAISE EXCEPTION 'CHECK FAILED: %', p_what; END IF;
    RAISE NOTICE 'ok  %', p_what;
END $$;

CREATE FUNCTION pg_temp.cents(p_counterparty text) RETURNS bigint
LANGUAGE sql AS $$
    SELECT outstanding_cents FROM invoice_overview
    WHERE counterparty_name LIKE p_counterparty || '%' AND direction = 'receivable' AND status = 'sent'
    ORDER BY issue_date DESC LIMIT 1;
$$;

-- One skipped day, as the backend's mock bank and reminder worker would do it.
CREATE FUNCTION pg_temp.simulate_day() RETURNS void
LANGUAGE plpgsql AS $$
DECLARE
    v_acct uuid := (SELECT id FROM bank_accounts WHERE is_primary);
    v_bal  bigint;
    r      record;
BEGIN
    UPDATE company SET clock_offset_days = clock_offset_days + 1;
    SELECT available_cents INTO v_bal FROM current_balance;

    -- Cash flows that happen today.
    FOR r IN SELECT * FROM forecast_events(0) WHERE kind = 'cashflow' LOOP
        INSERT INTO bank_transactions (bank_account_id, boc_transaction_id, booked_date, dc_ind, amount_cents,
                                       description, cashflow_id)
        VALUES (v_acct, 'sim-' || r.cashflow_id || '-' || r.on_date, r.on_date,
                CASE WHEN r.amount_cents < 0 THEN 'debit' ELSE 'credit' END, r.amount_cents, r.label, r.cashflow_id);
        v_bal := v_bal + r.amount_cents;
    END LOOP;

    -- Bills due today are paid.
    FOR r IN SELECT * FROM invoice_overview
             WHERE direction = 'payable' AND status = 'sent' AND due_date = kerdos_today() LOOP
        INSERT INTO payments (invoice_id, amount_cents, method, provider) VALUES (r.id, r.outstanding_cents, 'bank_transfer', 'mock');
        v_bal := v_bal - r.outstanding_cents;
    END LOOP;

    INSERT INTO balance_snapshots (bank_account_id, available_cents, current_cents, source)
    VALUES (v_acct, v_bal, v_bal, 'mock');

    -- Reminders due today go out.
    INSERT INTO outbound_messages (kind, mode, invoice_id, sequence, tone, policy_id, recipient, subject, body,
                                   status, sent_at, provider, dedupe_key)
    SELECT d.kind, 'automated', d.invoice_id, d.sequence, d.tone, d.policy_id, c.email,
           'Reminder: invoice ' || d.number, '(rendered from template)', 'sent', now(), 'mock', d.dedupe_key
    FROM reminders_due d
    JOIN invoices i ON i.id = d.invoice_id
    JOIN counterparties c ON c.id = i.counterparty_id;
END $$;

-- ---------------------------------------------------------------- Day 0
SELECT pg_temp.check(kerdos_today() = (now() AT TIME ZONE 'Europe/Nicosia')::date, 'day 0 is the real local date');
SELECT pg_temp.check((SELECT available_cents FROM current_balance) = 100000, 'balance is €1,000');
SELECT pg_temp.check(
    (SELECT (first_negative_day, lowest_day, shortfall_cents) = (4, 12, 80000::bigint) FROM forecast_shortfall()),
    'forecast: first dip day 4, lowest −€800 on day 12');
SELECT pg_temp.check(
    NOT EXISTS (SELECT 1 FROM forecast_events() WHERE label LIKE 'Elena%'),
    'Elena (pays ~45 days late) is expected outside the 30-day window');
SELECT pg_temp.check(
    (SELECT day FROM forecast_events() WHERE label LIKE 'Andreas%') = 25,
    'Andreas (overdue, pays ~30 days late) is expected on day 25');
SELECT pg_temp.check(NOT EXISTS (SELECT 1 FROM reminders_due), 'no reminders due on day 0');

-- "Invoice Kostas €120": automatic invoicing, as the backend would write it.
DO $$
DECLARE v_id uuid;
BEGIN
    INSERT INTO invoices (number, direction, counterparty_id, description, amount_cents, vat_cents,
                          issue_date, due_date, status, source, source_text, provider, external_id, sent_at)
    SELECT next_invoice_number(extract(year FROM kerdos_today())::smallint), 'receivable', c.id, 'Services',
           12000, 0, kerdos_today(), kerdos_today() + co.payment_terms_days, 'sent', 'chat',
           'Invoice Kostas €120', 'mock', 'mock-kostas', now()
    FROM counterparties c CROSS JOIN company co
    WHERE 'Kostas' = ANY (c.aliases)
    RETURNING id INTO v_id;
    INSERT INTO invoice_lines (invoice_id, position, description, quantity, unit, unit_price_cents)
    VALUES (v_id, 1, 'Services', 1, 'service', 12000);
    INSERT INTO outbound_messages (kind, mode, invoice_id, recipient, subject, body, status, sent_at, provider, dedupe_key)
    SELECT 'invoice', 'automated', v_id, c.email, 'Invoice', '…', 'sent', now(), 'mock', 'invoice:' || v_id || ':sent'
    FROM invoices i JOIN counterparties c ON c.id = i.counterparty_id WHERE i.id = v_id;
END $$;
SET CONSTRAINTS ALL IMMEDIATE;
SET CONSTRAINTS ALL DEFERRED;

SELECT pg_temp.check(
    (SELECT number FROM invoices WHERE source = 'chat') = 'KRD-' || extract(year FROM kerdos_today()) || '-0006',
    'Kostas gets the next number, KRD-<year>-0006');
SELECT pg_temp.check(
    (SELECT day FROM forecast_events() WHERE label LIKE 'Kostas%') = 14,
    'Kostas has no history, so his €120 is expected on the due date, day 14');
SELECT pg_temp.check(
    (SELECT (lowest_day, shortfall_cents) = (12, 80000::bigint) FROM forecast_shortfall()),
    '"In 12 days you''ll be €800 negative" still holds');

-- ---------------------------------------------------------------- Skip time: +3 days
SELECT pg_temp.simulate_day();
SELECT pg_temp.simulate_day();
SELECT pg_temp.simulate_day();

SELECT pg_temp.check((SELECT clock_offset_days FROM company) = 3, 'clock moved 3 days');
SELECT pg_temp.check((SELECT available_cents FROM current_balance) = 130000, 'deposit arrived: balance €1,300');
SELECT pg_temp.check(
    (SELECT count(*) FROM outbound_messages WHERE kind = 'overdue' AND business_date > kerdos_today() - 3) = 1,
    'exactly one automatic reminder during the skip');
SELECT pg_temp.check(
    (SELECT (c.name, m.sequence, m.tone, m.business_date - kerdos_today())
            = ('Andreas Electric'::text, 2, 'firm'::text, -2)
     FROM outbound_messages m JOIN invoices i ON i.id = m.invoice_id JOIN counterparties c ON c.id = i.counterparty_id
     WHERE m.kind = 'overdue' AND m.business_date > kerdos_today() - 3),
    'it is Andreas, reminder 2, firm, sent on day 1 (3 days after reminder 1)');
SELECT pg_temp.check(NOT EXISTS (SELECT 1 FROM reminders_due), 'nothing else is due: no duplicates');
SELECT pg_temp.check(
    (SELECT (first_negative_day, lowest_day, shortfall_cents) = (1, 9, 80000::bigint) FROM forecast_shortfall()),
    '"In 9 days you''ll be €800 negative"');
SELECT pg_temp.check(
    (SELECT (last_tone, reminders_sent) = ('firm'::text, 2::bigint) FROM debtors WHERE counterparty_name LIKE 'Andreas%'),
    'debtors view: Andreas, 2 reminders, last tone firm');

-- ---------------------------------------------------------------- Get €800 now
-- A queued nudge for Elena, to prove that accepting financing cancels it.
INSERT INTO outbound_messages (kind, mode, invoice_id, recipient, subject, body, dedupe_key)
SELECT 'upcoming', 'automated', id, 'elena@example.com', 'Due soon', '…', 'test:elena:upcoming'
FROM invoices WHERE description = 'Menu and logo design';

WITH run AS (
    INSERT INTO forecast_runs (as_of_date, horizon_days, start_balance_cents, points, first_negative_date,
                               lowest_date, lowest_balance_cents, shortfall_cents)
    SELECT kerdos_today(), 30, (SELECT available_cents FROM current_balance),
           (SELECT jsonb_agg(jsonb_build_object('date', on_date, 'balance_cents', balance_cents) ORDER BY day) FROM forecast_points()),
           kerdos_today() + s.first_negative_day, kerdos_today() + s.lowest_day, s.lowest_balance_cents, s.shortfall_cents
    FROM forecast_shortfall() s
    RETURNING id, shortfall_cents
), offer AS (
    INSERT INTO financing_offers (forecast_run_id, product, amount_cents)
    SELECT id, 'invoice_financing', shortfall_cents FROM run
    RETURNING id, amount_cents
), picked AS (  -- newest first, until the invoices cover the amount
    SELECT o.id AS invoice_id, o.outstanding_cents,
           sum(o.outstanding_cents) OVER (ORDER BY o.issue_date DESC, o.id) - o.outstanding_cents AS covered_before
    FROM invoice_overview o
    WHERE o.direction = 'receivable' AND o.status = 'sent' AND NOT o.is_financed AND o.outstanding_cents > 0
)
INSERT INTO financing_offer_invoices (financing_offer_id, invoice_id, outstanding_cents)
SELECT offer.id, picked.invoice_id, picked.outstanding_cents
FROM offer, picked WHERE picked.covered_before < offer.amount_cents;

SELECT pg_temp.check(
    (SELECT sum(outstanding_cents) FROM financing_offer_invoices) = 95000,
    'offer is backed by Kostas, Elena and Andreas (€950)');
SELECT pg_temp.check(accept_financing_offer((SELECT id FROM financing_offers)), 'accepting the offer works');
SELECT pg_temp.check(NOT accept_financing_offer((SELECT id FROM financing_offers)), 'a second click changes nothing');

-- Simulated disbursal: the money arrives.
UPDATE financing_offers SET status = 'disbursed', disbursed_at = now();
INSERT INTO balance_snapshots (bank_account_id, available_cents, current_cents, source)
SELECT bank_account_id, available_cents + 80000, current_cents + 80000, 'simulated' FROM current_balance;

SELECT pg_temp.check((SELECT available_cents FROM current_balance) = 210000, 'balance €2,100 after the advance');
SELECT pg_temp.check(NOT EXISTS (SELECT 1 FROM forecast_shortfall()), 'the warning clears');
SELECT pg_temp.check((SELECT min(balance_cents) FROM forecast_points()) = 0, 'lowest point is exactly €0 on day 12');
SELECT pg_temp.check(
    (SELECT count(*) FROM invoice_overview WHERE display_status = 'financed') = 3, 'three invoices show as financed');
SELECT pg_temp.check(NOT EXISTS (SELECT 1 FROM debtors), 'financed invoices leave the debtors list');
SELECT pg_temp.check(
    (SELECT status FROM outbound_messages WHERE dedupe_key = 'test:elena:upcoming') = 'cancelled',
    'financing cancelled Elena''s queued nudge');
UPDATE company SET clock_offset_days = clock_offset_days + 30;
SELECT pg_temp.check(NOT EXISTS (SELECT 1 FROM reminders_due), 'financed invoices are never reminded, even 30 days later');
UPDATE company SET clock_offset_days = clock_offset_days - 30;

-- ---------------------------------------------------------------- The client pays (pay by bank)
WITH k AS (SELECT id, number, amount_cents FROM invoices WHERE source = 'chat'),
pay AS (
    INSERT INTO payments (invoice_id, amount_cents, method, provider, external_id)
    SELECT id, amount_cents, 'pay_by_bank', 'boc', '12345678' FROM k RETURNING id
)
INSERT INTO payment_initiations (invoice_id, provider, payment_id, creditor_iban, amount_cents, payment_details,
                                 status, ref_number, executed_at, resulting_payment_id)
SELECT k.id, 'boc', '1ccec876-2440-48d7-9751-7ea57935335e', 'CY00 sample', k.amount_cents,
       k.number || ' Nicosia Studio', 'executed', '12345678', now(), pay.id
FROM k, pay;
SELECT pg_temp.check(
    (SELECT (status, display_status) = ('paid'::text, 'paid'::text) FROM invoice_overview WHERE source = 'chat'),
    'Kostas paid: invoice marked paid automatically');

-- ---------------------------------------------------------------- Safety rules
DO $$ BEGIN
    BEGIN
        INSERT INTO invoices (number, direction, counterparty_id, amount_cents, issue_date, due_date, status, source, source_text)
        SELECT 'BAD-1', 'receivable', id, 99999, kerdos_today(), kerdos_today(), 'sent', 'chat', 'x'
        FROM counterparties WHERE name LIKE 'Kostas%';
        INSERT INTO invoice_lines (invoice_id, position, description, quantity, unit_price_cents)
        SELECT id, 1, 'x', 1, 100 FROM invoices WHERE number = 'BAD-1';
        SET CONSTRAINTS ALL IMMEDIATE;
        RAISE EXCEPTION 'mismatched totals were accepted';
    EXCEPTION WHEN check_violation THEN
        RAISE NOTICE 'ok  an invoice whose total differs from its lines is rejected';
    END;
    SET CONSTRAINTS ALL DEFERRED;
END $$;

DO $$ BEGIN
    BEGIN
        INSERT INTO outbound_messages (kind, mode, invoice_id, sequence, tone, recipient, subject, body, status, sent_at)
        SELECT 'overdue', 'manual', invoice_id, sequence, tone, recipient, subject, body, 'sent', now()
        FROM outbound_messages WHERE kind = 'overdue' AND sequence = 2 LIMIT 1;
        RAISE EXCEPTION 'a duplicate reminder number was accepted';
    EXCEPTION WHEN unique_violation THEN
        RAISE NOTICE 'ok  the same reminder number cannot be sent twice';
    END;
END $$;

DO $$ BEGIN
    BEGIN
        INSERT INTO bank_transactions (bank_account_id, boc_transaction_id, booked_date, dc_ind, amount_cents)
        SELECT id, 'bad-sign', kerdos_today(), 'debit', 500 FROM bank_accounts LIMIT 1;
        RAISE EXCEPTION 'a debit with a positive amount was accepted';
    EXCEPTION WHEN check_violation THEN
        RAISE NOTICE 'ok  a BoC debit must be negative';
    END;
END $$;

DO $$ BEGIN
    BEGIN
        UPDATE invoices SET status = 'cancelled' WHERE description = 'Menu and logo design';
        INSERT INTO payments (invoice_id, amount_cents, method) SELECT id, 100, 'cash' FROM invoices WHERE description = 'Menu and logo design';
        RAISE EXCEPTION 'a payment on a cancelled invoice was accepted';
    EXCEPTION WHEN check_violation THEN
        RAISE NOTICE 'ok  a cancelled invoice cannot be paid';
    END;
END $$;

\echo 'ALL DEMO STORY CHECKS PASSED'
ROLLBACK;
