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
- **Google sign-in** (OAuth 2.0 / OpenID Connect). The backend holds all tokens; the browser only gets a session cookie. Only the `owner` role exists for now.
- **Invoices page with two directions:** "To the company" (payable, from suppliers) and "From the company" (receivable). Search, filter, sort.
- **Google Sheets sync and export** (CSV/XLSX, "Open in Google Sheets").
- **Gmail reminders, manual and automated:** editable templates with placeholders such as `{client_name}`, `{invoice_number}`, `{amount}`, `{due_date}`; optional Claude draft that the owner approves; every send logged, no duplicate sends.
- **Spending forecast by category against a monthly budget,** with an alert above 80%. The cash-gap forecast above stays the headline; this is a second view on the same data.

### Status of the web app
The Next.js app in `web/` runs on the Go backend's API: overview, chat invoicing, invoices (both directions, CSV export), debtors, the cash forecast with the financing offer, and reminders. See `web/README.md` for what is not built yet.

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
- BoC developer portal and sandbox: https://developer.bankofcyprus.com (accounts, balances, transactions, payments via SEPA/SWIFT).
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
