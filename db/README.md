# Kerdos database

PostgreSQL 17 in Docker. The Go backend owns it; the schema lives here as versioned migrations in the [golang-migrate](https://github.com/golang-migrate/migrate) format (`NNNNNN_name.up.sql` / `.down.sql`).

## Setup

Create `db/.env` (git-ignored, never commit it):

```
KERDOS_DB_PASSWORD=<choose a password>
KERDOS_DB_PORT=5432
```

`KERDOS_DB_PORT` is optional (default 5432). The database listens on `127.0.0.1` only.

Run from the repo root:

```sh
docker compose -f db/compose.yaml --env-file db/.env up -d db                      # start
docker compose -f db/compose.yaml --env-file db/.env run --rm migrate up           # apply migrations
docker compose -f db/compose.yaml --env-file db/.env exec -T db \
  psql -U kerdos -d kerdos -v ON_ERROR_STOP=1 --single-transaction < db/seed/sample.sql   # sample data (once)
```

Optional: pgAdmin (web UI) at http://127.0.0.1:5050, no login. The `Kerdos` server is pre-registered; it asks for `KERDOS_DB_PASSWORD` on first connect.

```sh
docker compose -f db/compose.yaml --env-file db/.env --profile tools up -d pgadmin
```

Connection string for the Go backend: `postgres://kerdos:<password>@127.0.0.1:5432/kerdos?sslmode=disable`

Reset to empty: `docker compose -f db/compose.yaml --env-file db/.env down -v` (deletes the data volume).

## Conventions

- Money: integer cents (`bigint`). `invoices.amount_cents` is the total including VAT; `vat_cents` is the VAT part.
- Dates are `date`, instants are `timestamptz`. Sample seed dates are relative to the day it runs.
- Single company: `company` holds exactly one row.
- `reminder_sends` is both the send log and the job queue. Workers claim `queued` rows with `FOR UPDATE SKIP LOCKED`; `dedupe_key` makes automated sends idempotent.
- Google tokens are stored encrypted by the backend (`key_version` supports key rotation). Session tokens are stored as hashes.
- Receivable numbers come from `next_invoice_number(year)`, e.g. `KRD-2026-0001`. The format is a proposal.
