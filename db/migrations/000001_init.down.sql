DROP TABLE IF EXISTS audit_log, financing_requests, reminder_sends, reminder_rules, budgets,
    bank_transactions, bank_accounts, invoice_drafts, invoice_lines, invoices, invoice_counters,
    counterparties, sessions, google_tokens, users, company;
DROP FUNCTION IF EXISTS next_invoice_number(smallint);
