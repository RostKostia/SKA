# Kerdos — Project Brief (for agents)

## Context
- Event: Bank of Cyprus Fintech Hackathon 7.0, 27–29 Nov 2026, IDEA Center, Nicosia. https://bochackathon.net/en/
- Team: "Kerdos" (same name as the product), 3 people. The team speaks Russian.
- Track: #4 Fintech for SMEs & Entrepreneurship (also touches #3 Open Banking & Embedded Finance).
- Prizes: €7k / €4k / €2k, plus €2k for Best use of Jinius APIs (a key target).
- Rules: a working MVP is required. The code must be in a public repo (CC BY-SA).
- Judging: innovation, technical soundness, automated data visualisation, use of open APIs, feasibility, and commercial/social impact.

## Problem
Small businesses in Cyprus (cafés, tradespeople, freelancers) don't fail for lack of customers. They fail because money comes in later than it goes out.
1. Late payments: invoices are sent over WhatsApp or paper, and clients pay 45–90 days late. Chasing them is awkward and slow.
2. Cash-flow gaps: owners find out the account will go negative only when a payment already fails (rent, salaries, 13th salary, insurance, seasonal winter drop).

## Solution: "Kerdos" (κέρδος, Greek for profit, gain)
An AI assistant for small businesses, built on Claude and integrated with Jinius (BoC's B2B platform: e-invoicing, payments, marketplace) and the BoC open banking APIs.

### MVP features
1. **Invoice in 30 seconds.** From natural language, such as "Invoice Kostas, 3 hours at €40", it creates an e-invoice and sends it via Jinius with a payment link, plus a copy by email.
2. **Reminders.** It reminds the debtor every N days, with the tone escalating (polite → firm). The owner gets a summary of who owes what.
3. **Cash forecast.** From the account balance (BoC API) plus unpaid and upcoming invoices: "In 12 days you'll be €800 negative."
4. **Action plan.** Remind specific clients, or get money now against an unpaid invoice (BoC overdraft or invoice financing).

### Features added after the first brief
From the Kerdos feature request. The ones the backend does not serve yet are listed in `web/README.md`.
- **Sign-in with email and password, or Google** (OAuth 2.0 / OpenID Connect). The backend holds all tokens; the browser only gets a session cookie. Roles: the owner (the first account) and members the owner invites.
- **Invoices page with two directions:** "To the company" (payable, from suppliers) and "From the company" (receivable). Search, filter, sort.
- **Google Sheets sync and export** (CSV/XLSX, "Open in Google Sheets"). Only the CSV export exists so far.
- **Gmail reminders, manual and automated:** editable templates with placeholders such as `{client_name}`, `{invoice_number}`, `{amount}`, `{due_date}`; optional Claude draft that the owner approves; every send logged, no duplicate sends.
- **Spending forecast by category against a monthly budget,** with an alert above 80%. The cash-gap forecast above stays the headline; this is a second view on the same data.

### Status (Sat 28 Nov)
Working end to end on the Go backend, in both the web app (`web/`) and the mockup (served at `/mockup/`):
- Chat invoicing, automatic and manual reminders, debtors, the cash forecast, "Get €800 now", and the demo controls (Skip time, Reset). The client's pay page with "Pay now" is in the mockup and the API, not yet in the web app.
- **Email:** real over SMTP, tested with Gmail and an app password. Chat and the mockup say an email went out only once the mail server accepted it; a failed one says why and has a Retry button. Nothing is emailed about a cancelled invoice, and paid or financed invoices are never chased.
- **Accounts:** sign in with email and password or Google (`KERDOS_AUTH=accounts`). The first account owns the business and invites the others by email; account page (profile, password, Google link, sign out everywhere) and team page (invite, cancel, remove). In the web app and the mockup. The default, `KERDOS_AUTH=dev`, has no sign-in, for the demo and tests.
- **Claude** reads chat messages when `ANTHROPIC_API_KEY` is set. Otherwise, or on any failure, the rule-based parser does.
- **BoC open banking:** consent, accounts, balance and statement (the Accounts API) are built, and an incoming payment that mentions an invoice number marks that invoice paid. So far this is tested only against the sample responses in the BoC documentation. It has not been run against the live sandbox yet.

Simulated: Jinius delivery (no public docs), "Pay now" (the BoC Payments API isn't built yet), and financing offers and payout (BoC has no lending API).

Not built: Google Sheets sync, XLSX export (the web app exports CSV), budgets and the category forecast, editing reminder templates, Claude-drafted reminders, and a bank connection page in the web app.

Details: the Status table in `README.md`, and `web/README.md` for the web app.

### Open items
- Run the BoC connection against the live sandbox from a team laptop (steps in `backend/README.md`).
- Google Sheets: the proposal for the demo is a service account that writes to one shared spreadsheet. Giving each user their own sheet would also ask them for Sheets access, on top of Google sign-in. Not decided yet.
- Confirm Jinius access with the organisers.
- Google sign-in needs a Google OAuth client (ID and secret in `.env`); steps in `backend/README.md`. Decide whether the demo runs with sign-in on.

### Architecture
- **Frontend:** Next.js (App Router, TypeScript) in `web/`. It only displays data and proxies to the backend. Every outside service sits behind a mock in the backend, so the demo never depends on a service being up.
- **Backend:** Go owns all logic, endpoints, integrations (BoC, Jinius, Google, Claude) and the database.
- **Contract:** the endpoint table in `backend/README.md`; `web/lib/api/types.ts` mirrors its JSON.
- **Data model:** `docs/data-model.md` (team-agreed). Config only through env vars; no secrets in the repo.

### Out of scope
- Physical cash in the till: dropped, because the bank can't see it and it's niche.
- WhatsApp channel: later. Email only for the MVP.

### Nice-to-have (only if time allows)
- Translation for expats (Greek / English / Russian).
- Client reliability score (who usually pays late).
- "What-if" chat ("Can I afford to hire someone?").

## Value for the bank
- Invoices and payments flow through Jinius and BoC accounts.
- The cash-gap forecast gives a natural moment to sell credit (overdraft / invoice financing).
- It brings new SME clients into BoC.

## APIs
- BoC developer portal and sandbox: https://developer.bankofcyprus.com (accounts, balances, transactions, payments via SEPA/SWIFT). Kerdos uses the Accounts API: subscription consent, accounts, balance and statement. The Payments API ("pay by bank") is planned for "Pay now". The sandbox settings are in `.env.example`.
- Jinius: https://www.bankofcyprus.com/en-gb/wholesale/Digital-solutions/digitize-your-business/Jinius2/. The Marketplace API uses an account API key. Public docs for e-invoicing were not found. **Confirm access with the organisers.**
- If an API is unavailable, use a mock or adapter behind the same interface and fall back to demo data. The demo must never break.

## Demo script (2 min)
1. The owner says or types: "Invoice Kostas €120". The invoice arrives on a second phone in the room.
2. Time skips forward and an automatic reminder is sent.
3. Kerdos: "In 12 days you'll be €800 negative. Get €800 against Kostas's invoice now?" Press the button, done.

## Plan
- **Fri evening:** confirm API access, get sandbox keys, create a public GitHub repo, make one test payment.
- **Sat:** invoice → Jinius, AI parsing, reminders, debt summary, forecast, financing button, UI (chat, invoice list, forecast chart), demo end to end.
- **Sun morning:** 5 slides (problem → Kerdos → demo → bank value → scale), rehearse twice, record a backup demo video, check the public repo.

## Principles
- 3 working features beat 6 broken ones.
- If an API blocks progress for more than about 1 hour, switch to mock data.
- Keep the stack simple and easy to deploy.
