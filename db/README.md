# Kerdos database

PostgreSQL 17 in Docker. The Go backend owns it; the schema lives here as versioned migrations in the [golang-migrate](https://github.com/golang-migrate/migrate) format (`NNNNNN_name.up.sql` / `.down.sql`). The logical model and the team decisions behind it are in [`docs/data-model.md`](../docs/data-model.md).

## Setup

Create `db/.env` (git-ignored, never commit it):

```
KERDOS_DB_PASSWORD=<choose a password>
KERDOS_DB_PORT=5432
```

`KERDOS_DB_PORT` is optional (default 5432). Set it to 5433 when another PostgreSQL already holds 5432 on your computer, and use the same port in `KERDOS_DATABASE_URL`. The database listens on `127.0.0.1` only.

Run from the repo root:

```sh
docker compose -f db/compose.yaml --env-file db/.env up -d db                      # start
docker compose -f db/compose.yaml --env-file db/.env run --rm migrate up           # apply migrations
docker compose -f db/compose.yaml --env-file db/.env exec -T db \
  psql -U kerdos -d kerdos -v ON_ERROR_STOP=1 --single-transaction < db/seed/sample.sql   # sample data (once)
docker compose -f db/compose.yaml --env-file db/.env exec -T db \
  psql -U kerdos -d kerdos -v ON_ERROR_STOP=1 < db/tests/demo_story.sql                   # check the demo story
```

On Windows PowerShell, see [Windows (PowerShell)](../project-description.md#windows-powershell) in the project description: commands with `<` need `cmd /c`, and `curl` needs `Invoke-RestMethod` or `curl.exe`.

The last command walks the whole 2-minute demo and the safety rules against the sample data, prints one `ok` line per check and ends with `ALL DEMO STORY CHECKS PASSED`. It runs in a transaction that is rolled back, so it changes nothing.

Before the demo, point Kostas at the second phone (the full checklist is in [project-description.md](../project-description.md#the-2-minute-demo)):
`UPDATE counterparties SET email = '<demo phone email>' WHERE name LIKE 'Kostas%';`

Optional: pgAdmin (web UI) at http://127.0.0.1:5050, no login. The `Kerdos` server is pre-registered; it asks for `KERDOS_DB_PASSWORD` on first connect.

```sh
docker compose -f db/compose.yaml --env-file db/.env --profile tools up -d pgadmin
```

Connection string for the Go backend: `postgres://kerdos:<password>@127.0.0.1:<KERDOS_DB_PORT>/kerdos?sslmode=disable` (5432 by default, 5433 if you changed it).

Reset to empty: `docker compose -f db/compose.yaml --env-file db/.env down -v` (deletes the data volume). Do this once if you applied the earlier version of `000001_init`, since it was rewritten before anything shipped.

## Conventions

- Money: integer cents (`bigint`). `invoices.amount_cents` is the total including VAT; `vat_cents` is the VAT part. An invoice with lines must add up to them; this is checked at commit.
- Dates are `date`, instants are `timestamptz`. Enums are `text` + `CHECK`. Ids are `uuid`.
- Single company: `company` holds exactly one row.
- **Today is `kerdos_today()`**: the local date in `company.timezone` plus `company.clock_offset_days`. The demo's "Skip time" adds to the offset; everything that depends on the date (overdue, reminders, forecast) reads this function, never `current_date`. Rows that belong to a business day (`payments.received_on`, `outbound_messages.business_date`, `balance_snapshots.business_date`) default to it.
- `invoices.status` is `draft`, `sent`, `paid` or `cancelled`. "Overdue", "financed" and "partially paid" are derived: read `invoice_overview.display_status`.
- Chat invoices are sent at once (team decision): `draft` only lasts while the send to Jinius is in flight, or after it failed.
- `outbound_messages` is the send log and the job queue for every email: invoice delivery, nudges before the due date, overdue reminders, receipts, the owner's summary and budget alerts. Workers claim `queued` rows with `FOR UPDATE SKIP LOCKED`; `dedupe_key` makes automated sends idempotent; an overdue reminder number can only be live once per invoice.
- Google tokens are stored encrypted by the backend (`key_version` supports key rotation). Session tokens are stored as hashes. The BoC app token is not stored (it is per app and lasts 60 minutes); a `bank_connections` row holds the subscription (the owner's consent).
- Receivable numbers come from `next_invoice_number(year)`, e.g. `KRD-2026-0001`. Numbers are never reused; a wrong invoice is cancelled and a new one issued.

## What the database computes

| Name | Kind | Gives |
|---|---|---|
| `invoice_overview` | view | Every invoice with `paid_cents`, `outstanding_cents`, `is_financed`, `display_status`, `days_overdue` |
| `debtors` | view | Who owes what: open, non-financed receivables per client, reminders sent, last tone |
| `counterparty_payment_stats` | view | Each client's `avg_days_late` from their paid invoices (negative = early) |
| `reminders_due` | view | Reminders to send today, with `sequence`, `tone` and `dedupe_key`. Overdue reminder 1 is due `first_after_due_days` after the due date, each next one `every_days` after the last one sent; tone follows `tone_steps` (polite → firm → final). Financed and paid invoices never appear |
| `current_balance` | view | Latest balance snapshot of the primary account |
| `forecast_events(horizon)` | function | Money in and out from tomorrow on: receivables on `max(due + client's delay, tomorrow)`, payables on their due date, cash-flow occurrences. Financed invoices are left out |
| `forecast_points(horizon)` | function | Projected balance for each day 0…horizon |
| `forecast_shortfall(horizon)` | function | `first_negative_day`, `lowest_day`, `shortfall_cents`: "In 12 days you'll be €800 negative". No row when there's no shortfall |
| `accept_financing_offer(offer, user)` | function | Accepts once (a double click returns false), marks the backing invoices financed and cancels their queued reminders |
| `payments_settle_invoice` | trigger | A payment that covers the invoice marks it paid and cancels its queued reminders |

## "Skip time" (demo)

For each skipped day, the backend: adds 1 to `company.clock_offset_days`; lets the mock bank post that day's money (cash flows due today as `bank_transactions` with `cashflow_id`, bills due today as `payments`) and write a new `balance_snapshots` row; then turns each `reminders_due` row into an `outbound_messages` row. `pg_temp.simulate_day()` in `tests/demo_story.sql` does exactly this and is the reference.
