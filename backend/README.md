# Kerdos backend (Go prototype)

A JSON API over the PostgreSQL schema in [`../db`](../db). It runs the whole demo with mock adapters, so it needs no keys: chat invoicing, reminders, the cash forecast, "Get €800 now" and the client's pay page. Claude, real email (SMTP) and the BoC sandbox switch on through settings.

Go 1.22+, standard library plus one dependency (`github.com/jackc/pgx/v5`, the PostgreSQL driver).

## Run it

1. Start the database and load the sample data, as in [`db/README.md`](../db/README.md) (start, migrate, seed).
2. Copy `.env.example` to `.env` in the repo root, and set `KERDOS_DATABASE_URL` with your `KERDOS_DB_PASSWORD`.
3. From `backend/`:

   ```sh
   go mod tidy                # first time only: downloads pgx and writes go.sum
   go run ./cmd/kerdos-api    # listens on :8080
   ```

4. Check: `curl http://localhost:8080/healthz` should give `{"ok":true,"db":"ok",...}`.

The API reads `.env` from `backend/` or the repo root. Real environment variables win.

## The demo through the API

```sh
curl -X POST localhost:8080/api/demo/reset                         # same story every rehearsal
curl localhost:8080/api/forecast                                    # "In 12 days you'll be €800 negative."
curl -X POST localhost:8080/api/chat -d '{"text":"Invoice Kostas €120"}'
curl -X POST localhost:8080/api/demo/skip -d '{"days":3}'           # deposit arrives, Andreas gets a firm reminder
curl -X POST localhost:8080/api/financing/offers                    # €800 against Kostas, Elena and Andreas
curl -X POST localhost:8080/api/financing/offers/<offer id>/accept  # the warning clears
curl localhost:8080/api/pay/<pay token>                             # the client's page (token is in the invoice's payment_link_url)
curl -X POST localhost:8080/api/pay/<pay token>                     # "Pay now"
```

On Windows PowerShell, see [Windows (PowerShell)](../project-description.md#windows-powershell) in the project description: commands with `<` need `cmd /c`, and `curl` needs `Invoke-RestMethod` or `curl.exe`.

## Settings

| Variable | Default | What it does |
|---|---|---|
| `KERDOS_DATABASE_URL` | `postgres://kerdos:kerdos@127.0.0.1:5432/kerdos?sslmode=disable` | The database |
| `KERDOS_HTTP_ADDR` | `:8080` | Where the API listens |
| `KERDOS_PUBLIC_URL` | `http://localhost:3000` | The frontend. Pay links point to `<this>/pay/<token>`; BoC consent returns to `<this>/settings` (not built in the web app yet) |
| `KERDOS_DEMO` | `1` | Enables `/api/demo/skip` and `/api/demo/reset`. Set it to `0` outside the demo |
| `KERDOS_SEED_FILE` | `../db/seed/sample.sql` | What reset loads |
| `KERDOS_MOCKUP_DIR` | `../mockup` | The clickable mockup, served at `/mockup/` (and `/` redirects there) when the folder exists. `off` turns it off |
| `KERDOS_AUTH` | `dev` | `dev`: no sign-in, every request acts as the owner (handy for curl and tests). `accounts`: sign-in required, see [Accounts and sign-in](#accounts-and-sign-in) |
| `KERDOS_WORKER_INTERVAL` | `1m` | How often the worker queues due reminders and sends emails |
| `KERDOS_MOCK_FINANCING_FEE_BP` | `150` | The sample fee on simulated offers (1.5%) |
| `ANTHROPIC_API_KEY` | empty | When set, Claude reads chat messages. Empty, or any failure, falls back to the rule-based parser |
| `ANTHROPIC_MODEL` | `claude-haiku-5-5` | |
| `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASSWORD`, `SMTP_FROM` | empty, `587` | Real email. For Gmail use `smtp.gmail.com` with an app password. Empty means emails are only logged. A send gives up after 10 s to connect, 30 s in all |
| `KERDOS_BANK` | `mock` | `boc` uses the BoC sandbox, with the `BOC_*` settings below |
| `BOC_BASE_URL`, `BOC_CLIENT_ID`, `BOC_CLIENT_SECRET`, `BOC_REDIRECT_URI` | sandbox URL | `BOC_REDIRECT_URI` must be `http://localhost:8080/api/bank/callback`, registered exactly like that on the BoC portal |
| `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` | empty | "Continue with Google". Empty means email and password only |
| `GOOGLE_REDIRECT_URI` | `http://localhost:8080/api/auth/google/callback` | Must be listed exactly like that under the Google client's authorized redirect URIs |

## Endpoints

All bodies are JSON. Money is in integer cents (`*_cents`) and dates are `YYYY-MM-DD`. Errors look like `{"error":{"code":"…","message":"…"}}`, with codes such as `not_found`, `no_shortfall`, `already_reminded_today`, `already_decided` and `not_failed`.

| Method and path | What it does |
|---|---|
| `GET /healthz` | Database, bank, mailer and demo status |
| `GET /api/auth/status` | **Public.** `mode` (`dev` or `accounts`), `needs_setup` (nobody can sign in yet), `google`, and the signed-in `user` or null |
| `POST /api/auth/signup {name, email, password, invite}` | **Public.** Creates an account and signs in. Needs an invitation token, except for the very first account, which becomes the owner |
| `POST /api/auth/login {email, password}` · `POST /api/auth/logout` | **Public.** Sign in (wrong passwords are rate-limited) and out. Signing in sets the `kerdos_session` cookie |
| `GET /api/auth/google/start?return_to=&invite=` · `POST /api/auth/google/start {return_to, invite}` | **Public.** Continue with Google: a redirect, or `{url}`. `return_to` must be on `KERDOS_PUBLIC_URL` or this server |
| `GET /api/auth/google/callback` | **Public.** Google comes back here; the browser then goes to `return_to?login_code=…`, or `?auth_error=…&auth_message=…` |
| `POST /api/auth/exchange {code}` | **Public.** Swaps the one-time `login_code` for a session |
| `GET /api/auth/invites/{token}` | **Public.** Who an invitation is for, for the sign-up page |
| `POST /api/auth/logout-everywhere` | Ends every session of the signed-in user |
| `GET /mockup/` | The clickable mockup, running on this API (same origin, so no CORS) |
| `GET /api/me` · `PATCH /api/me {name, email, current_password}` | The signed-in user, with `has_password` and `google_linked`. Changing the email of an account with a password needs that password |
| `POST /api/me/password {current_password, new_password}` | Changes the password (or sets a first one for a Google-only account) and signs out the other sessions |
| `POST /api/me/google/link {return_to}` → `{url}` · `DELETE /api/me/google` | Links a Google account, or disconnects it (only with a password, so nobody locks themselves out) |
| `GET /api/team` · `POST /api/team/invites {email, name}` · `DELETE /api/team/invites/{id}` · `DELETE /api/team/members/{id}` | **Owner only.** Members and open invitations; invite (emails the link and returns it as `link`), cancel an invitation, remove a member |
| `GET /api/dashboard` | Side panel: balance, forecast headline and points, who owes what, action plan |
| `GET /api/chat` · `POST /api/chat {text}` | Chat history; send a message. Replies carry a `card` (`invoice`, `reminder`, `warning`, `offer`, `financed`, `paid`, `need_details`, `suggestions`) and the card's current `invoice` |
| `GET /api/invoices?direction=receivable\|payable&status=overdue…` | Invoices with `display_status`, `outstanding_cents`, `is_financed` |
| `POST /api/invoices {counterparty_id, lines[]}` | Manual invoice. It is sent at once, like chat invoices. The reply's `email` is the copy's real outcome (`sent`, or `failed` with an `error`) |
| `GET /api/invoices/{id}` | One invoice with its lines, payments and emails |
| `POST /api/invoices/{id}/remind` | Manual reminder: the next overdue one, or a nudge before the due date. One per day. Returns the email with its real `status`; if today's reminder failed, this sends it again |
| `POST /api/invoices/{id}/cancel` | Cancel an open invoice; its number is not reused |
| `GET /api/counterparties` · `POST /api/counterparties {name, email, aliases}` | Clients and suppliers |
| `GET /api/debtors` | Who owes what (open, non-financed invoices) |
| `GET /api/forecast?horizon=30` | Daily points, events, `shortfall`, `headline`, `method` |
| `POST /api/financing/offers` · `GET …/{id}` · `POST …/{id}/accept` | Offer to cover the shortfall; accepting works once and pays out (simulated) |
| `GET /api/messages?invoice_id=&kind=` | The email log |
| `POST /api/messages/{id}/retry` | Send a failed email again, at once (after fixing the SMTP settings, say). Only `failed` emails |
| `GET /api/reminders/settings` | The reminder policy and the email templates |
| `POST /api/demo/skip {days}` · `POST /api/demo/reset` | The demo's "Skip time", and a reset to the sample story. Reset keeps accounts; with `KERDOS_AUTH=accounts` only the owner may reset |
| `GET /api/pay/{token}` · `POST /api/pay/{token}` | **Public.** The client's invoice page and "Pay now" (simulated pay by bank) |
| `POST /api/bank/connect` → `{authorize_url}` · `GET /api/bank/callback` · `POST /api/bank/sync` | BoC consent and sync, with `KERDOS_BANK=boc` |

## Emails

Every email is a row in `outbound_messages`, which is both the log and the send queue. Just before sending, Kerdos checks the invoice again: an email for a cancelled invoice is never sent, and neither is a reminder for an invoice that has been paid or financed. Those rows become `cancelled`, with the reason in `error`.

Chat and the mockup only say an email went out once the mail server accepted it. If it failed, the reply says why (a refused Gmail login, for example) and the mockup shows a Retry button. A failed email stays `failed` until you retry it; nothing resends it on its own.

## Accounts and sign-in

With `KERDOS_AUTH=accounts`, every `/api` route except the public ones above needs a session. There is one business:

- **The first account becomes the owner.** While nobody can sign in, `GET /api/auth/status` says `needs_setup`, and the first sign-up (password or Google) takes over the seed's placeholder owner. So create the owner account as soon as you turn accounts on, before anyone else can reach the server.
- **Everyone else joins by invitation.** The owner invites an address from the Team page. Kerdos emails a link that works once, for 7 days, and only for that address. Someone using Google with that verified address can join without the link.
- **Password or Google, or both.** Passwords are stored as PBKDF2-SHA256 hashes (600,000 iterations). Google sign-in uses the authorization code flow with PKCE, asks only for name and email, and links to an existing account with the same verified address.
- **Sessions** are random tokens, stored as hashes, valid for 30 days. The browser gets an httpOnly `kerdos_session` cookie (SameSite=Lax), and the API refuses state-changing requests that a browser sends from another website. The web app's server forwards the cookie; other clients can send `Authorization: Bearer <token>`.
- **Removing a member** stops them signing in and ends their sessions. Their history stays. Invited back, they get their old account.

To turn on Google sign-in:

1. In the Google Cloud console, open Google Auth Platform, register the app if asked, then go to Clients, choose Create client and pick Web application.
2. Under Authorized redirect URIs, add `http://localhost:8080/api/auth/google/callback` exactly (it can take a few minutes to apply).
3. Copy the client ID and secret into `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET` in `.env`. The secret is shown only once.
4. While the app is in testing, add the team's Google accounts as test users (on the Audience page).

## Connecting the BoC sandbox

With `KERDOS_BANK=boc` and the `BOC_*` settings in `.env` (see `.env.example`), Kerdos reads a BoC sandbox account through the Accounts API:

1. `curl -X POST localhost:8080/api/bank/connect` returns an `authorize_url`. In PowerShell: `Invoke-RestMethod -Method Post http://localhost:8080/api/bank/connect`.
2. Open that link in a browser and sign in with BoC's sandbox test user (`BOC_SANDBOX_USERNAME` and `BOC_SANDBOX_PASSCODE` in `.env.example`). Then approve the accounts.
3. BoC sends the browser back to `/api/bank/callback`. Kerdos activates the subscription, saves the accounts (the first current account becomes the primary one) and syncs the balance and the last 90 days of the statement. An incoming payment that mentions an invoice number marks that invoice paid.
4. The backend then redirects to `KERDOS_PUBLIC_URL/settings?bank=connected` (or `?bank=error`). The web app has no `/settings` page yet, so you'll see "not found" there, but the connection is already saved.
5. `POST /api/bank/sync` fetches again whenever you want. Nothing syncs on its own.

The consent lasts 180 days. Once connected, the forecast starts from the sandbox balance, so the demo's "€800 negative" numbers change. `POST /api/demo/reset` removes the connection and goes back to the sample story. For the 2-minute demo, use `KERDOS_BANK=mock`.

## Layout

```
cmd/kerdos-api/      main: settings, adapters, worker, HTTP server
internal/app/        business operations and SQL (accounts, chat, invoices, reminders, forecast, financing, demo, pay page, bank)
internal/httpapi/    routes, JSON, errors; demo_test.go walks the whole demo through the API
internal/parse/      rule-based chat parser (the mockup's rules, plus intents)
internal/ai/         Claude parser (tool use) with automatic fallback to the rules
internal/boc/        BoC PSD2 client: token, subscription consent, accounts, balance, statement
internal/auth/       password hashing (PBKDF2) and random tokens
internal/google/     Sign in with Google (OpenID Connect, PKCE)
internal/jinius/     invoice delivery interface; mock only (no public Jinius e-invoicing docs)
internal/mail/       log-only mailer and SMTP mailer
internal/money/      euro formatting and parsing, in cents
```

The rules live in the database (`invoice_overview`, `reminders_due`, `forecast_*`, `accept_financing_offer`; see `db/README.md`), so the API and the SQL test in `db/tests/demo_story.sql` can't disagree.

## Tests

```sh
go test ./...                                  # unit tests: parser, money, Claude fallback, BoC client, SMTP against a fake mail server, passwords, Google sign-in
```

The end-to-end test resets the database it is given, so point it at a separate one:

```sh
docker compose -f db/compose.yaml --env-file db/.env exec -T db createdb -U kerdos kerdos_test
docker compose -f db/compose.yaml --env-file db/.env exec -T db psql -U kerdos -d kerdos_test -v ON_ERROR_STOP=1 < db/migrations/000001_init.up.sql
docker compose -f db/compose.yaml --env-file db/.env exec -T db psql -U kerdos -d kerdos_test -v ON_ERROR_STOP=1 < db/migrations/000002_accounts.up.sql
KERDOS_TEST_DATABASE_URL="postgres://kerdos:<password>@127.0.0.1:5432/kerdos_test?sslmode=disable" go test ./internal/httpapi/
```

The end-to-end tests are `demo_test.go` (the whole demo), `email_test.go` (failed and retried emails, cancelled invoices, reminders that fail) and `accounts_test.go` (sign-up, sign-in, invitations, the account page, the team, and Google sign-in against a fake Google).

The BoC client is tested against a fake server that replays the sample responses from the BoC documentation, including transactions with no `dcInd`. It has not been run against the live sandbox yet.

## Real, simulated, not built yet

- **Real:** the database rules; accounts with email and password or Google sign-in; chat parsing (rules, or Claude with a key); email over SMTP, with each email's real outcome reported; BoC consent, balance and statement sync, with incoming payments matched by invoice number (not yet run against the live sandbox).
- **Simulated:** Jinius delivery (the payment link is Kerdos's own pay page); "Pay now" (no signed BoC payment initiation yet); financing offers and payout; the bank movements during "Skip time" in mock mode.
- **Not built yet:**
  - The Gmail API (SMTP works with Gmail meanwhile), and roles beyond owner and member
  - Google Sheets sync and XLSX export (the web app builds its CSV export from `GET /api/invoices`)
  - Budget and category endpoints
  - BoC payment initiation
  - `api/openapi.yaml`, which the frontend spec expects: this README's endpoint table is the contract until then
