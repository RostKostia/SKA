# Kerdos Step 1: Contract, Next.js Scaffold and Design Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver step 1 of the six: the API contract mirroring the Postgres schema, a Next.js app that talks to a typed client (mock by default, Go over HTTP when configured), a tactile and animated design foundation, and the renamed, updated brief.

**Architecture:** `api/openapi.yaml` is the single contract for the Go backend. `web/` is a Next.js App Router app whose server code reaches the backend only through one `ApiClient` interface, with a mock implementation (default) and an HTTP implementation. The browser never talks to Go except through a Next.js rewrite for the Google sign-in redirect. Business logic, validation and persistence stay in Go.

**Tech Stack:** Next.js (App Router), React, TypeScript, Vitest (dev), `openapi-typescript` (dev), CSS and Web Animations API for motion (no library), PostgreSQL 17 schema already in `db/`.

**Spec:** `docs/superpowers/specs/2026-11-27-kerdos-frontend-design.md`. Data model source: `db/migrations/000001_init.up.sql`.

## Global Constraints

- Dependencies: Next.js, React, TypeScript, Vitest (dev), `openapi-typescript` (dev), plus what `create-next-app` installs (including ESLint). Nothing else without approval; no motion library.
- No business logic, database access, token handling, Google/BoC/Jinius/Claude calls in Next.js.
- Browser code never holds Google tokens, API keys or bank credentials. Config only through env vars; no secrets in the public repo.
- Money is integer cents everywhere (`amountCents`), currency `EUR`; dates are `YYYY-MM-DD` strings; instants are RFC 3339.
- Mock data is always labelled "sample" in the UI; the UI never implies it is real.
- WCAG AA contrast, body text at least 16 px, `prefers-reduced-motion` respected, no motion that blocks input. English UI.
- Env vars: `KERDOS_API_MODE` (`mock` default, or `http`), `KERDOS_API_URL` (required when `http`).
- Git (user rules): work on the current branch `prototype_mock` unless told otherwise; never commit without proposing 2-3 messages and waiting for the user's pick; no co-author trailers. Every "Commit" step below means exactly that.
- `mockup/` stays untouched as the visual and behavioral reference.

## Review Focus

Failure modes the spec implies but no feature task would otherwise test, most likely first:
1. **Backend unreachable or slow:** the UI gets a clear `ApiError` (`unreachable`), never a blank page or an unhandled rejection. Pinned in Task 6.
2. **Expired or missing session (401):** surfaces as `ApiError` with `status 401`, which the shell turns into a redirect to sign-in. Pinned in Task 6 and Task 8.
3. **Date-only strings shifting a day** in timezones behind UTC (`new Date("2026-11-27")`). Pinned in Task 4.
4. **Greek names, accents and final sigma in search** ("μαρια" must find "Μαρία Σάββα"; empty or whitespace query returns everything). Pinned in Task 5.
5. **Negative and zero money** (cash-gap forecast shows `-€800.00`, not `€-800.00` or `-800`). Pinned in Task 4.

---

### Task 1: Rename and update the brief

**Files:**
- Rename: `KUMBARES_BRIEF.md` to `KERDOS_BRIEF.md` (`git mv`)
- Modify: `KERDOS_BRIEF.md`

- [ ] **Step 1:** `git mv KUMBARES_BRIEF.md KERDOS_BRIEF.md`.
- [ ] **Step 2:** In `KERDOS_BRIEF.md`, replace the name Kumbares with Kerdos (κέρδος, "profit") and add, without deleting the original product text: a short **Architecture** section (Next.js frontend and proxy in `web/`, all logic and endpoints in Go, PostgreSQL schema in `db/`, contract in `api/openapi.yaml`, mock mode default), a **Features added** section listing Google login with roles, invoices page with two directions, Sheets sync and export, Gmail reminders (manual and automated) and the category-versus-budget forecast, and a note that the cash-gap forecast stays the headline with the budget forecast as a second view. Keep the brief's "Principles".
- [ ] **Step 3: Verify.** Run `grep -n "Kumbares" KERDOS_BRIEF.md`. Expected: no matches except an optional single "(formerly Kumbares)" line.
- [ ] **Step 4: Commit** `KERDOS_BRIEF.md` (propose 2-3 messages, wait for the user).

### Task 2: Scaffold the Next.js app with Vitest and the type generator

**Files:**
- Create: `web/` (via `create-next-app`), `web/vitest.config.ts`
- Modify: `web/package.json`, `web/README.md`

**Interfaces:**
- Produces: npm scripts `dev`, `build`, `lint`, `test` (`vitest run`), `gen:api` (`openapi-typescript ../api/openapi.yaml -o lib/api/schema.d.ts`); import alias `@/*` to `web/`.

- [ ] **Step 1:** From the repo root run `npx create-next-app@latest web --ts --app --eslint --no-tailwind --no-src-dir --import-alias "@/*" --use-npm` (accept no extra prompts; no Tailwind, no `src/`).
- [ ] **Step 2:** In `web/`, run `npm i -D vitest openapi-typescript`.
- [ ] **Step 3:** Add `vitest.config.ts` (node environment, `@` alias resolving to the `web/` directory, include `**/*.test.ts`) and the `test` and `gen:api` scripts.
- [ ] **Step 4:** Replace the generated `web/README.md` with a short Kerdos one: what `web/` is, `npm run dev`, the two env vars above, that mock mode needs no backend.
- [ ] **Step 5: Verify.** `cd web && npm run lint && npm run build`. Expected: both succeed.
- [ ] **Step 6: Commit** (propose messages, wait).

### Task 3: The OpenAPI contract and generated types

**Files:**
- Create: `api/openapi.yaml`, `web/lib/api/schema.d.ts` (generated, committed), `web/lib/api/types.ts`

**Interfaces:**
- Produces: `types.ts` exporting `Session`, `User`, `Invoice`, `InvoiceQuery`, `InvoiceDraft`, `ReminderRule`, `ReminderSend`, `Budget`, `Forecast`, `ActionItem`, `ApiErrorBody` as aliases of `components["schemas"][...]`.

- [ ] **Step 1: Write `api/openapi.yaml`** (OpenAPI 3.1, `servers: /v1`). Auth is the httpOnly cookie `kerdos_session` (proposal). Every non-2xx returns `ApiErrorBody {code: string, message: string}`. Schemas mirror `db/` in camelCase: money `amountCents`/`vatCents` (integer), `currency`, dates `issueDate`/`dueDate`, enums `direction: payable|receivable`, `status: draft|sent|paid|overdue|cancelled`, `source: manual|jinius|chat|boc`, `role: admin|user`, reminder `kind: upcoming|overdue|budget`, `mode: manual|automated`, send `status: queued|sent|failed`. `Session` is `{user, company{name,currency,vatRatePercent}, dataSource: sample|live}`. `Invoice` embeds `counterparty{id,name,email}`. `Forecast` has `method{name,description,months}`, `cashGap{points[{date,balanceCents}], firstNegativeDate?, shortfallCents?}`, `categories[{category, actualCents[], forecastCents, budgetCents?}]`, `alerts[{level: warn|over, category, message}]`. Operations (operationId, method, path):

| operationId | method path |
|---|---|
| getSession | GET /session |
| startGoogleSignIn | GET /auth/google/start (302 redirect) |
| logout | POST /auth/logout |
| listUsers | GET /users (admin) |
| setUserRole | PUT /users/{id}/role (admin) |
| listInvoices | GET /invoices (query: direction, status, q, sort, order) |
| getInvoice | GET /invoices/{id} |
| createInvoice | POST /invoices |
| cancelInvoice | POST /invoices/{id}/cancel |
| exportInvoices | GET /invoices/export (query: format csv or xlsx, direction) |
| createInvoiceDraft | POST /chat/invoice-drafts (body: text) |
| approveInvoiceDraft | POST /chat/invoice-drafts/{id}/approve |
| syncSheets | POST /sheets/sync |
| listReminderRules | GET /reminders/rules |
| updateReminderRule | PUT /reminders/rules/{id} |
| draftReminder | POST /reminders/draft |
| sendReminders | POST /reminders/send |
| listReminderLog | GET /reminders/log |
| listBudgets | GET /budgets (query: month) |
| setBudget | PUT /budgets/{month}/{category} |
| getForecast | GET /forecast (query: months) |
| listActions | GET /actions |
| createFinancingRequest | POST /financing-requests |
| skipTime | POST /demo/skip-time (sample mode only) |

  `InvoiceQuery.sort` is one of `dueDate`, `issueDate`, `amountCents`, `number`, `counterparty`; `order` is `asc` or `desc`; default `dueDate asc`.
- [ ] **Step 2:** `cd web && npm run gen:api`. Expected: succeeds and writes `lib/api/schema.d.ts`. A spec error fails here; fix it until clean.
- [ ] **Step 3:** Write `web/lib/api/types.ts` with the aliases listed under Interfaces.
- [ ] **Step 4: Verify.** `npx tsc --noEmit`. Expected: no errors.
- [ ] **Step 5: Commit** (propose messages, wait).

### Task 4: Formatting helpers

**Files:**
- Create: `web/lib/format.ts`
- Test: `web/lib/format.test.ts`

**Interfaces:**
- Produces: `formatMoney(cents: number, currency: string): string`; `formatDate(isoDate: string): string`.

- [ ] **Step 1: Write failing tests** in `format.test.ts`. Set `process.env.TZ = "Pacific/Honolulu"` at the top of the file with a one-line comment that it proves date-only strings do not shift. Assertions: `formatMoney(123450, "EUR") === "€1,234.50"`; `formatMoney(-80000, "EUR") === "-€800.00"`; `formatMoney(0, "EUR") === "€0.00"`; `formatDate("2026-11-27") === "27 Nov 2026"`; `formatDate("2026-03-01") === "1 Mar 2026"`.
- [ ] **Step 2:** `cd web && npx vitest run lib/format.test.ts`. Expected: FAIL (module missing).
- [ ] **Step 3:** Implement with `Intl.NumberFormat("en-IE", {style: "currency", currency})` dividing cents by 100, and `Intl.DateTimeFormat("en-GB", {day:"numeric", month:"short", year:"numeric", timeZone:"UTC"})` applied to `Date.UTC(y, m-1, d)` parsed from the string parts.
- [ ] **Step 4:** Re-run. Expected: PASS.
- [ ] **Step 5: Commit** (propose messages, wait).

### Task 5: ApiClient interface and the mock

**Files:**
- Create: `web/lib/api/client.ts`, `web/lib/api/mock.ts`
- Test: `web/lib/api/mock.test.ts`

**Interfaces:**
- Consumes: `Session`, `Invoice`, `InvoiceQuery` from `types.ts`.
- Produces: `interface ApiClient { getSession(): Promise<Session>; listInvoices(query?: InvoiceQuery): Promise<Invoice[]> }` and `class ApiError extends Error { status: number; code: string }` in `client.ts`; `createMockApi(opts?: { today?: string }): ApiClient` in `mock.ts`. Later steps add methods named exactly as the contract's operationIds.

- [ ] **Step 1: Write failing tests** in `mock.test.ts` using `createMockApi({today: "2026-11-27"})`:
  - `getSession()` returns `dataSource "sample"`, role `admin`, currency `EUR`, company name containing "(sample)".
  - `listInvoices()` returns both directions; `{direction: "payable"}` returns only payable; `{status: "overdue"}` returns only overdue.
  - default order is `dueDate` ascending; `{sort: "amountCents", order: "desc"}` puts the largest amount first.
  - `{q: "elena"}` matches "Elena's Bakery (sample)"; `{q: "ELENA"}` too; `{q: "   "}` returns all invoices; `{q: "μαρια"}` returns the invoice whose counterparty is "Μαρία Σάββα (sample)" (seed one such client, so Greek accents and final sigma are exercised).
  - an unknown `q` returns `[]`.
- [ ] **Step 2:** Run `npx vitest run lib/api/mock.test.ts`. Expected: FAIL.
- [ ] **Step 3:** Implement `client.ts` and `mock.ts`. Seed (in `mock.ts`, dates relative to `today`, cents): receivable Cyprus Tiles 60000 paid; Kostas Hadjis 12000 overdue; Elena's Bakery 45000 sent; Andreas Electric 38000 sent; Μαρία Σάββα 30000 sent; payable Studio Landlord 150000 sent; Cyprus Insurance 60000 sent. Numbers like `KRD-2026-0001`; names end with "(sample)". Search normalizes with `normalize("NFD")`, strips combining marks, lowercases and maps `ς` to `σ` on both sides. `today` defaults to the current date in `Europe/Nicosia`.
- [ ] **Step 4:** Re-run. Expected: PASS.
- [ ] **Step 5: Commit** (propose messages, wait).

### Task 6: HTTP client, config and sign-in rewrite

**Files:**
- Create: `web/lib/api/http.ts`, `web/lib/api/config.ts`, `web/lib/api/index.ts`
- Modify: `web/next.config.ts`
- Test: `web/lib/api/http.test.ts`, `web/lib/api/config.test.ts`

**Interfaces:**
- Consumes: `ApiClient`, `ApiError` (Task 5).
- Produces: `createHttpApi(opts: { baseUrl: string; getCookie?: () => Promise<string | undefined>; fetch?: typeof fetch }): ApiClient`; `resolveApiConfig(env: Record<string, string | undefined>): { mode: "mock" } | { mode: "http"; baseUrl: string }`; `buildRewrites(env): { source: string; destination: string }[]`; `getApi(): ApiClient` (server only, reads `process.env`, passes the `kerdos_session` cookie via `next/headers`).

- [ ] **Step 1: Write failing tests.**
  - `http.test.ts` (inject a fake `fetch`): requests go to `${baseUrl}/v1/session` and `/v1/invoices?direction=payable&q=...` with the query URL-encoded; the `Cookie` header is forwarded from `getCookie`; a 401 with body `{code:"unauthenticated",message:"x"}` throws `ApiError` with `status 401` and `code "unauthenticated"`; a 500 with a non-JSON body throws `ApiError` with `code "unexpected"`; a rejected `fetch` throws `ApiError` with `status 0` and `code "unreachable"`.
  - `config.test.ts`: `resolveApiConfig({})` and `{KERDOS_API_MODE:"mock"}` give `{mode:"mock"}`; `{KERDOS_API_MODE:"http",KERDOS_API_URL:"http://go:8080"}` gives `{mode:"http", baseUrl:"http://go:8080"}`; `http` without a URL throws an error whose message contains `KERDOS_API_URL`; any other mode value throws; `buildRewrites` returns `[]` in mock mode and one rule `/backend/:path*` to `<url>/:path*` in http mode.
- [ ] **Step 2:** `npx vitest run lib/api`. Expected: FAIL.
- [ ] **Step 3:** Implement. `http.ts` maps non-2xx responses to `ApiError` and has no retries (the Go side owns resilience). `next.config.ts` calls `buildRewrites(process.env)` inside `rewrites()`. The sign-in link in the UI is `/backend/v1/auth/google/start`, so the session cookie is set on the app's own origin.
- [ ] **Step 4:** Re-run, then `npm run build`. Expected: tests PASS and build succeeds in default mock mode.
- [ ] **Step 5: Commit** (propose messages, wait).

### Task 7: Design foundation: tokens, tactile components, motion and the `/design` page

**Files:**
- Create: `web/app/globals.css`, `web/app/design/page.tsx`, `web/components/ui/Button.tsx`, `web/components/ui/Surface.tsx`, `web/components/ui/Paper.tsx`, `web/components/ui/StatusStamp.tsx`
- Modify: `web/app/layout.tsx`
- Test: `web/tests/contrast.test.ts`, `web/tests/motion.test.ts`

**Interfaces:**
- Produces: CSS custom properties in `:root` named `--paper` (page background), `--surface`, `--ink`, `--ink-muted`, `--accent`, `--accent-ink` (text on accent), `--danger`, `--danger-ink`; duration and easing tokens `--dur-fast`, `--dur-base`, `--dur-slow`, `--ease-out`, `--ease-spring`; components `Button({variant: "primary"|"quiet", ...})`, `Surface`, `Paper` (paper-like invoice slip), `StatusStamp({status: Invoice["status"]})`.

Design skills apply here, in this order: `frontend-design` for the visual identity (more tactile and more unique than the mockup, a distinct Kerdos look, not a templated fintech dashboard), `animation-studio` for motion, then `design:accessibility-review` as the check.

- [ ] **Step 1: Write failing tests.**
  - `contrast.test.ts` reads `app/globals.css`, parses the `:root` hex values and asserts WCAG contrast of at least 4.5 for these pairs: (`--ink`, `--paper`), (`--ink`, `--surface`), (`--ink-muted`, `--paper`), (`--ink-muted`, `--surface`), (`--accent-ink`, `--accent`), (`--danger-ink`, `--danger`).
  - `motion.test.ts` asserts `globals.css` contains an `@media (prefers-reduced-motion: reduce)` block that sets animation and transition durations to near zero.
- [ ] **Step 2:** Run `npx vitest run tests`. Expected: FAIL (file missing).
- [ ] **Step 3:** Build the design with the skills above. Tactile means layered shadows and depth, pressable controls with distinct rest, hover, pressed and focus-visible states, and a paper-like invoice slip. Motion means a rise-in on mount, a press and release spring on buttons, a stamp animation when a status chip changes, and a line-draw keyframe for the later forecast chart, built only with CSS and the Web Animations API. Fonts load through `next/font` in `layout.tsx`. The `/design` page shows palette, type scale, every component state and each motion sample, labelled "design kit", for the review pause.
- [ ] **Step 4:** Re-run `npx vitest run tests`. Expected: PASS. Fix tokens, not the tests, if contrast fails.
- [ ] **Step 5:** `npm run lint && npm run build`. Then run `npm run dev` and screenshot `/design` at 1440 px wide, once normally and once with `prefers-reduced-motion: reduce` emulated, to confirm motion stops.
- [ ] **Step 6: Commit** (propose messages, wait).

### Task 8: App shell and home page wired to the client

**Files:**
- Create: `web/components/Shell.tsx`
- Modify: `web/app/layout.tsx`, `web/app/page.tsx`, `web/README.md`

**Interfaces:**
- Consumes: `getApi()` (Task 6), `formatMoney`/`formatDate` (Task 4), UI components (Task 7).

- [ ] **Step 1:** `Shell.tsx`: left nav (Chat, Invoices, Debtors, Forecast, Reminders, entries only, pages come in later steps), top bar with the company name from `getSession()` and a visible **Sample data** badge when `dataSource` is `sample`. If `getApi().getSession()` throws `ApiError` with status 401, redirect to `/backend/v1/auth/google/start`; for any other `ApiError`, render a clear error panel with the message and a retry link.
- [ ] **Step 2:** `app/page.tsx` shows a welcome panel and the open invoice count and total owed, read through `getApi().listInvoices({direction: "receivable"})`, formatted with `formatMoney`.
- [ ] **Step 3:** Update `web/README.md` with the contract, DB and mock-mode pointers.
- [ ] **Step 4: Verify.** `npm run lint && npm test && npm run build`. Then `npm run dev` and screenshot `/` and `/design`. Confirm the Sample data badge is visible and the home page shows the receivable total. Then set `KERDOS_API_MODE=http` with `KERDOS_API_URL=http://127.0.0.1:9` and confirm the error panel appears instead of a crash.
- [ ] **Step 5: Commit** (propose messages, wait).

---

## Steps 2 to 6 (own plans, written after review)

Each gets a detailed plan only after the previous step's pause, because the Go team's feedback on the contract shapes them: 2 Google sign-in UI and sessions, 3 invoices page and chat invoicing, 4 Sheets sync and export, 5 reminders UI, 6 forecast and budget alerts.

## Self-review

- **Spec coverage:** brief rename (Task 1), Next.js plus Vitest plus generator (2), contract and data model (3), client seam with mock default and http mode (5, 6), sign-in rewrite (6), tactile and motion design with skills (7), shell with sample badge and 401 handling (8). Steps 2-6 UI is deferred by design. The `db/` schema exists already.
- **Type consistency:** `ApiClient`, `ApiError`, `createMockApi`, `createHttpApi`, `resolveApiConfig`, `buildRewrites`, `getApi` are named identically wherever they appear.
- **Review Focus:** all five lines have a test in Tasks 4 to 6, or the Task 8 verification for the 401 redirect.
