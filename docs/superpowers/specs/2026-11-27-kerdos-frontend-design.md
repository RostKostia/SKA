# Kerdos frontend and API contract: design

Status: approved by the user on 2026-11-27, with the changes recorded below (Go-only backend, tactile and unique look with motion, Vitest and a type generator allowed). Not committed.

## Intent and why
Kumbares becomes **Kerdos** (κέρδος, "profit"), the real product. A Go backend owns all business logic, the database, every endpoint and every integration (BoC, Jinius, Google, Claude). This repo's job is the **Next.js frontend** and the **API contract** those services implement. The frontend must work end to end against a mock backend, so the hackathon demo never depends on a service being ready.

## Goals
- A Next.js (App Router, TypeScript) app that covers the paste's features as UI: Google sign-in, invoices (payable and receivable), Sheets sync and export, reminders (manual and automated), spending forecast with budget alerts. It also covers the brief's chat invoicing, debtors summary and "get money now" action.
- One contract file (`api/openapi.yaml`) as the single source of truth for the Go backend.
- One typed API client module as the only code that talks to the backend. A mock implementation of it is the default.
- Clear seams for later work: backend handlers and API calls get added behind the client and the proxy without UI changes.
- `KUMBARES_BRIEF.md` becomes `KERDOS_BRIEF.md`, updated as decisions land (see "Brief updates").

## Non-goals
- No business logic, database, OAuth code exchange, token storage, scheduling, Gmail or Sheets calls, Claude calls or BoC/Jinius calls in Next.js.
- No Go code in this repo's first steps.
- No WhatsApp, till cash, language switching, reliability score or what-if chat (unchanged from the brief).

## Anti-goals
- Duplicating backend logic in TypeScript, so the two can drift.
- Browser code ever holding Google tokens, API keys or bank credentials.
- Implying mock data is real.

## Constraints
- Public repo (CC BY-SA): no secrets committed; config only through env vars.
- Dependencies: Next.js, React, TypeScript, Vitest (dev) and `openapi-typescript` (dev), all approved. Anything else, including a motion library, needs approval first.
- The mockup's English UI, layout and copy are the starting point. Its look is not binding (see Look and motion).
- Hackathon timing: 27–29 Nov. Three working features beat six broken ones.

## Approaches considered
- **A. Node backend with SQLite serving the mockup UI.** Fewest dependencies, but contradicts the decision that the Go backend owns the logic.
- **B. Next.js frontend plus proxy, contract-first (chosen).** The Go backend owns the logic and Next.js only displays and proxies it.
- **C. Python FastAPI.** Splits languages and abandons `logic.js`. Ruled out.
- Within B, the Next.js route handlers could hold real logic (reading 1). Ruled out by the user.

## Design

**Layout** (new, in `web/`; `mockup/` stays as the visual and behavioral reference):
- `db/`: the Go backend's PostgreSQL 17 schema (golang-migrate migrations), sample seed and Docker compose file. The frontend never connects to it.
- `api/openapi.yaml`: the contract.
- `web/app/`: pages (login, chat, invoices, debtors, forecast, reminders, client pay page) and `app/api/*` same-origin proxy route handlers.
- `web/lib/api/`: `client.ts` (the interface plus types mirroring the contract), `http.ts` (calls `KERDOS_API_URL`), `mock.ts` (in-memory, seeded from the mockup's sample data). `KERDOS_API_MODE=mock` is the default.

**Division of responsibility**

| Frontend (this repo) | Backend (Go, all endpoints) |
|---|---|
| Pages, forms, display, client-side input checks | Authoritative validation, authorization and roles |
| Redirect to the backend for sign-in; send the session cookie | Google OAuth code exchange, refresh, revocation, encrypted tokens |
| Invoice list, filters, search, sort | Invoice lifecycle and statuses, BoC matching, numbering, VAT |
| Buttons for sync, export and send | Sheets sync, CSV/XLSX export, Gmail send, queue, dedupe, limits, audit log |
| Charts and alerts from API data | Forecast and budget calculation |
| Approve or edit a draft | Claude drafting and invoice parsing |

**Auth flow** (my proposal): the browser goes to the backend's Google sign-in endpoint. The backend completes OAuth and sets an httpOnly session cookie. Next.js forwards the cookie through the proxy and never sees a Google token. Roles arrive from the backend's session endpoint and only control what the UI shows.

**Contract scope**: users and roles, invoices (direction, status, counterparty, amount, currency, issue date, due date), reminders (rules, templates with `{client_name}`, `{invoice_number}`, `{amount}`, `{due_date}`, send log), budgets (per month or category), forecast (cash-gap series plus category-versus-budget series with method description), Sheets sync and export, and chat invoice drafts. Contract-level choices are marked as proposals and are reviewed by the Go and PHP teams at the step-1 pause.

**Steps** (pause for review after each, as the paste asks)
1. Plan, data model and `openapi.yaml`, plus the Next.js scaffold with the mock client.
2. Google sign-in UI and session handling.
3. Invoices page and chat invoicing.
4. Sheets sync and export UI.
5. Reminders UI, manual and automated.
6. Forecast and budget alerts.

**Testing**: UI and client tests run against the mock. The forecast and templating unit tests belong to the Go backend. `mockup/logic.js` and its tests are handed to it as a reference for the forecast and reminder-tone rules; they are not ported to TypeScript.

**Brief updates**: during implementation, rename `KUMBARES_BRIEF.md` to `KERDOS_BRIEF.md` and update it with the name change, the Next.js plus Go architecture, the contract-first rule, the paste's features, and the merged forecast. Old Kumbares mockup docs under `docs/superpowers/` stay as history.

## My calls (change any of these)
- Project folder `web/`; TypeScript; App Router.
- Types are generated from `openapi.yaml` with `openapi-typescript` (approved), so they cannot drift from the contract.
- Tests run on Vitest (approved).
- Motion is built with CSS and the Web Animations API, with no library, and honors `prefers-reduced-motion`. A motion library would need approval.
- EUR and Cyprus standard VAT of 19% are display values from the backend. Invoice numbering is the backend's decision (proposal: `KRD-2026-0001`). Single company, no multi-tenancy.
- The Google Forms scope is dropped, since no feature uses it.
- Order within step 1: contract first, then scaffold.

## Look and motion (user's direction)
The mockup's design was accepted, but the user wants it **more tactile and more unique**, with **motion** included. Design work uses the installed skills: `frontend-design` for aesthetic direction, typography and distinctiveness, `animation-studio` for motion, and `design:accessibility-review` as a check. Tactile means surfaces that feel physical (depth, layered shadows, pressable controls with visible press and release states, paper-like invoice and receipt elements, tactile toggles and sliders). Unique means a clear visual identity for Kerdos rather than a templated fintech dashboard. Motion covers purposeful transitions (page and tab changes, invoice status changes, the forecast chart drawing in, the "skip time" step, the reminder send, the financing button confirmation). Constraints that stay: WCAG AA contrast, body text at least 16 px, `prefers-reduced-motion` respected, no motion that blocks input. The exact palette, type and motion language are decided in step 1 and shown to the user at that pause.

## Left to the builder
Exact copy, page layouts (following the mockup), chart drawing, mock seed values, and the proxy's error handling and retry behavior toward the backend.

## Open items
- The Go team reviews the contract at the step-1 pause.
