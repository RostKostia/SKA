BEGIN;

DROP FUNCTION IF EXISTS accept_financing_offer(uuid, uuid), forecast_shortfall(integer),
    forecast_points(integer), forecast_events(integer);
DROP VIEW IF EXISTS current_balance, reminders_due, debtors, invoice_overview, counterparty_payment_stats;

DROP TABLE IF EXISTS audit_log, chat_messages, outbound_messages, payment_initiations, payments,
    bank_transactions, cashflows, balance_snapshots, bank_accounts, bank_connections,
    financing_offer_invoices, invoice_lines, invoices, financing_offers, forecast_runs, budgets,
    invoice_counters, counterparties, message_templates, reminder_policies, sessions,
    google_tokens, users, company;

DROP FUNCTION IF EXISTS next_invoice_number(smallint), check_invoice_totals(),
    settle_invoice_on_payment(), set_updated_at(), kerdos_today();

COMMIT;
