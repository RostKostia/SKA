# Kerdos UI Rebuild Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild the `web/` UI as a dark toned colors as in "control room" on Tailwind and shadcn-style components, with a real forecast chart, `/login`, `/account`, professional copy and purposeful CSS motion.

**Architecture:** Design tokens are plain `:root` CSS variables in `app/globals.css`, mapped into Tailwind v4 with `@theme inline`, so CSS and the token tests share one source. UI primitives are hand-written shadcn-style files in `web/components/ui/` over the `radix-ui` package. Pages stay Server Components with their current data fetching and server actions; only the chart and the mobile nav are client components. Logic that tests need (status labels, KPIs, chart maths, initials) lives in small pure modules under `web/lib/`.

**Tech Stack:** Next.js 16.4.6, React 19.3.0, Tailwind CSS 4.3.5, radix-ui 1.8.1, Recharts 3.11.2, lucide-react 1.83.0, Vitest 5 (node environment, `*.test.ts` only).

**Spec:** `docs/superpowers/specs/2026-11-28-kerdos-ui-rebuild-design.md`

## Global Constraints

- Approved new dependencies, exactly these: `tailwindcss` and `@tailwindcss/postcss` 4.3.5, `radix-ui` 1.8.1, `class-variance-authority` 0.7.1, `clsx` 2.1.1, `tailwind-merge` 3.7.1, `lucide-react` 1.83.0, `recharts` 3.11.2, `tw-animate-css` 1.4.0. No motion library. Do not run `npx shadcn`; copy the component pattern by hand.
- Dark theme only. Motion is CSS only: 150–250 ms for UI feedback, ease-out entrances, nothing over 500 ms except the chart draw-in.
- One `@media (prefers-reduced-motion: reduce)` block sets `animation-duration` and `transition-duration` to `0.01ms` (keeps `tests/motion.test.ts` valid).
- Text contrast meets WCAG AA (4.5:1), checked by `tests/contrast.test.ts`.
- No change to `web/lib/api/*`, server actions, proxy routes or data fetching. The only Go change is the chat chip string in `backend/internal/app/chat.go:239`.
- Read the relevant guide in `web/node_modules/next/dist/docs/` before writing Next-specific code (`web/AGENTS.md`).
- Work on branch `ui-rebuild`. **Do not commit or push.** The user's rules require proposing 2–3 commit messages and waiting for a choice; the final task does that.
- Copy rules: noun-based titles, no first-person questions in UI labels, terms "invoice", "client", "receivable". The brand stays "Kerdos". Subsets `latin`, `greek`, `cyrillic` for the font.
- Verification target is the Docker API (`docker compose -f db/compose.yaml --env-file db/.env --profile app up -d api`), seeded demo data; there is no mock mode. Do not reload the seed or drop the db volume.

## Review Focus

1. Forecast that never goes negative: no shortfall, no danger fill, no crash when `shortfall` is `null` (Task 6 tests).
2. Forecast that starts or stays below zero: danger fill covers the whole series (Task 6 tests).
3. Forecast with no points or no events: empty state instead of a crashed chart (Task 6 tests).
4. Backend unreachable or returning 501: `ErrorPanel` renders inside the new shell, and `/login` still renders without the backend (Task 8 check).
5. Reduced motion on: the chart line is fully visible and no animation runs (Task 6 test on the CSS).
6. Long client names, large amounts (`€1,234,567.89`) and a 360px viewport: no horizontal page scroll; tables scroll inside their wrapper (Task 9 walkthrough).

---

### Task 1: Foundation: dependencies, tokens, fonts, motion base

**Files:**
- Modify: `web/package.json`, `web/package-lock.json` (via `npm install`)
- Create: `web/postcss.config.mjs`, `web/lib/utils.ts`
- Modify (rewrite): `web/app/globals.css`
- Modify: `web/app/layout.tsx`, `web/tests/contrast.test.ts`, `web/tests/motion.test.ts`

**Interfaces:**
- Produces: `cn(...inputs: ClassValue[]): string` in `lib/utils.ts` (clsx plus tailwind-merge). Tokens in `:root` as `--background`, `--surface`, `--surface-raised`, `--border`, `--foreground`, `--muted`, `--accent`, `--accent-foreground`, `--success`, `--warning`, `--danger`, `--danger-foreground`, `--info`, each a 6-digit hex. Utility classes `reveal` (fade-up entrance; stagger via inline `style={{ "--i": n }}`) and `draw-in` (stroke draw for the chart line). Tailwind colour names `bg-background`, `bg-surface`, `text-muted`, `border-border`, `bg-accent`, etc. via `@theme inline`.

- [ ] **Step 1: Branch and read docs.** Run `git switch -c ui-rebuild`. Read the Tailwind/CSS guide under `web/node_modules/next/dist/docs/`.
- [ ] **Step 2: Update the tests first.** In `tests/contrast.test.ts` replace `PAIRS` with: `["--foreground","--background"]`, `["--foreground","--surface"]`, `["--muted","--background"]`, `["--muted","--surface"]`, `["--muted","--surface-raised"]`, `["--accent-foreground","--accent"]`, `["--danger-foreground","--danger"]`, `["--accent","--surface"]`, `["--success","--surface"]`, `["--warning","--surface"]`, `["--danger","--surface"]`, `["--info","--surface"]`. Keep the test body. In `tests/motion.test.ts` keep the existing test and add `it("defines reveal and draw-in utilities")` asserting `css` matches `/\.reveal\b/` and `/\.draw-in\b/`.
- [ ] **Step 3: Run `npm test` in `web/`.** Expected: the contrast and motion tests FAIL (tokens and classes missing).
- [ ] **Step 4: Install.** In `web/` run `npm install tailwindcss@4.3.5 @tailwindcss/postcss@4.3.5 radix-ui@1.8.1 class-variance-authority@0.7.1 clsx@2.1.1 tailwind-merge@3.7.1 lucide-react@1.83.0 recharts@3.11.2 tw-animate-css@1.4.0`. Confirm only these appear as new top-level entries in `package.json`.
- [ ] **Step 5: Implement.** `postcss.config.mjs` exporting the `@tailwindcss/postcss` plugin. Rewrite `globals.css`: `@import "tailwindcss"; @import "tw-animate-css";`, then `:root` tokens (starting values from the spec: `#0b1220`, `#111a2e`, `#18233b`, `#25324d`, `#e6ecf7`, `#93a4c3`, `#2dd4bf`, plus status colours; tune until the contrast test passes), `@theme inline` mapping each to `--color-*`, the font variable, `@keyframes reveal` and `draw`, the `.reveal` and `.draw-in` classes, the reduced-motion block, and base styles (body background/foreground, `:focus-visible` ring using `--accent`, `font-variant-numeric: tabular-nums` on `.tabular`). Delete every old class (`.slab`, `.key`, `.slip`, `.stamp`, `.shell*`, `.table*`, `.chat*`, `.banner`, etc.); pages are rebuilt in later tasks. `lib/utils.ts` exports `cn`. In `app/layout.tsx` replace Commissioner and Source Serif with one `next/font/google` sans family (subsets `latin`, `greek`, `cyrillic`), and remove the `stamp-rough` SVG filter.
- [ ] **Step 6: Run `npm test`.** Expected: contrast and motion tests PASS; other suites unaffected. Run `npm run lint`; expected clean (old pages may still compile with unknown class names).

### Task 2: UI primitives and the `/design` page

**Files:**
- Create: `web/components/ui/{Button,Card,Badge,Table,Input,Tabs,Tooltip,Dialog,Skeleton}.tsx`, `web/lib/status.ts`, `web/lib/status.test.ts`
- Modify: `web/app/design/page.tsx`, `web/app/design/Demos.tsx`
- Delete (end of task): `web/components/ui/Paper.tsx`, `Surface.tsx`, `StatusStamp.tsx` once nothing imports them (Tasks 4–7 remove the imports; delete in Task 9 if still referenced).

**Interfaces:**
- Produces: `type Tone = "neutral" | "success" | "warning" | "danger" | "info"` and `statusMeta(status: DisplayStatus): { label: string; tone: Tone }` in `lib/status.ts`. Labels: draft "Draft", sent "Sent", cancelled "Cancelled", paid "Paid", financed "Financed", overdue "Overdue", partially_paid "Part paid", unpaid "Unpaid". Tones: paid→success, financed→info, overdue→danger, unpaid and partially_paid→warning, draft, sent and cancelled→neutral.
- Produces: `Button` (`variant: "primary" | "quiet" | "danger"`, `size: "sm" | "md"`, plus `asChild` via Radix Slot; keeps `type="button"` default), `Card`/`CardHeader`/`CardTitle`/`CardContent`, `Badge` (`{ tone: Tone }`), `StatusBadge` (`{ status: DisplayStatus; compact?: boolean }`, replaying a short scale transition when `key={status}` changes), `Table` family (`Table`, `THead`, `TBody`, `Tr`, `Th`, `Td` with a wrapper that scrolls horizontally), `Input`, `Tabs`, `Tooltip`, `Dialog`, `Skeleton`. Each accepts `className` merged with `cn`.

- [ ] **Step 1: Failing test** in `lib/status.test.ts`: for each of the 8 `DisplayStatus` values, assert `statusMeta(value)` equals the label and tone listed above.
- [ ] **Step 2: Run `npx vitest run lib/status.test.ts`.** Expected: FAIL (module missing).
- [ ] **Step 3: Implement `lib/status.ts`,** then the primitives. Server-safe components stay free of `"use client"`; Radix-based ones (`Tabs`, `Tooltip`, `Dialog`) are `"use client"`. Import from the `radix-ui` package (`import { Dialog } from "radix-ui"`).
- [ ] **Step 4: Rebuild `/design`** to show every primitive in each state (buttons incl. disabled, all badge tones, table, dialog trigger, skeleton). Add a section "Empty, error and shortfall states" that later tasks fill with fixtures.
- [ ] **Step 5: Run `npx vitest run lib/status.test.ts`** (PASS) and `npm run lint`.

### Task 3: App shell, page header, error and loading states

**Files:**
- Modify: `web/components/Shell.tsx`, `web/components/NavLinks.tsx`, `web/components/FlashBanner.tsx`, `web/components/ErrorPanel.tsx`, `web/components/PageFallback.tsx`, `web/app/(app)/layout.tsx`
- Create: `web/components/PageHeader.tsx`, `web/components/MobileNav.tsx`, `web/lib/nav.ts`, `web/lib/nav.test.ts`

**Interfaces:**
- Produces: `NAV_ITEMS: { href: string; label: string; icon: LucideIcon }[]` in `lib/nav.ts`, in order Overview `/`, Chat `/chat`, Invoices `/invoices`, Receivables `/debtors`, Forecast `/forecast`, Reminders `/reminders`. `ACCOUNT_HREF = "/account"`. `PageHeader({ title, description?, actions? })` renders the page `<h1>`. `MobileNav` is a `"use client"` Radix dialog opened by a menu button below 768px.
- Consumes: Task 2 primitives, `cn`.

- [ ] **Step 1: Failing test** `lib/nav.test.ts`: hrefs are unique, `/debtors` is labelled "Receivables", and no item's href equals `ACCOUNT_HREF`. (Pure `.ts` module: icons are values, so import is safe in node.)
- [ ] **Step 2: Run it.** Expected: FAIL.
- [ ] **Step 3: Implement.** `Shell` keeps its async role (fetches `getMe()` and the date as today), renders a fixed sidebar (brand, `NavLinks`, account entry linking to `/account` with initials and name), the top bar plus `MobileNav` under 768px, and `<main>` wrapping children. Restyle `FlashBanner`, `ErrorPanel` (keep `apiErrorView` and `role="alert"`), `PageFallback` (Skeleton blocks, `aria-busy="true"`). Do not change what `Shell` fetches.
- [ ] **Step 4: Run `npx vitest run lib/nav.test.ts`** (PASS), then `npm run build`; expected success.

### Task 4: Overview

**Files:**
- Modify: `web/app/(app)/page.tsx`
- Create: `web/lib/overview.ts`, `web/lib/overview.test.ts`

**Interfaces:**
- Produces: `overviewKpis(d: Dashboard): { owedCents: number; overdueClients: number; nextPayment: { date: string; label: string; amount_cents: number } | null }`. `owedCents` is `d.owed_cents`; `overdueClients` counts `d.debtors` with `max_days_overdue > 0`; `nextPayment` is the earliest `d.forecast.events` entry with `kind === "receivable"` and `amount_cents > 0`, else `null`.

- [ ] **Step 1: Failing tests** in `lib/overview.test.ts` with a fixture `Dashboard`: (a) two debtors, one with `max_days_overdue: 4` → `overdueClients === 1`; (b) events `[payable, receivable on day 3, receivable on day 9]` → `nextPayment` is the day-3 event; (c) no receivable events → `null`.
- [ ] **Step 2: Run it.** Expected: FAIL.
- [ ] **Step 3: Implement `overviewKpis`,** then rebuild the page: `PageHeader` titled "Overview"; a KPI row of three Cards (Outstanding receivables, Overdue clients, Next expected payment) using `formatMoney`/`formatDate`; below, the open invoices list with `StatusBadge`. Keep `OVERVIEW` Suspense/`PageFallback` structure and the `OPEN` status filter. Cards use `reveal` with `--i` 0..n.
- [ ] **Step 4: Run `npx vitest run lib/overview.test.ts`** (PASS), `npm run lint`.

### Task 5: Invoices and Receivables (debtors) pages

**Files:**
- Modify: `web/app/(app)/invoices/page.tsx`, `web/app/(app)/debtors/page.tsx`

**Interfaces:**
- Consumes: `PageHeader`, `Tabs`/links styled as tabs, `Table` family, `StatusBadge`, `Button`, `Input`. Direction tabs stay link-based (`aria-current="page"`), search and filters stay a GET form, server actions unchanged.

- [ ] **Step 1: Rebuild Invoices:** header "Invoices", direction tabs, toolbar (search `q`, status filter as today), table with number, client, due date, amount (right-aligned, tabular), status badge, row actions (remind, cancel) as small Buttons. Empty state card when no rows.
- [ ] **Step 2: Rebuild Debtors:** header title **"Outstanding receivables"**, table grouped by client: name, open invoices, outstanding, oldest age, reminders sent, last tone; empty state "No outstanding receivables." Keep the existing `metadata` titles but change "Debtors · Kerdos" to "Receivables · Kerdos".
- [ ] **Step 3: Verify:** `npm run lint` and `npm run build`; then load `/invoices` and `/debtors` against the Docker API and confirm rows, filters and the remind action still work.

### Task 6: Forecast chart and page

**Files:**
- Create: `web/components/ForecastChart.tsx` (replace the current file; `"use client"`), `web/lib/chart.ts`, `web/lib/chart.test.ts`
- Modify: `web/app/(app)/forecast/page.tsx`, `web/tests/motion.test.ts`

**Interfaces:**
- Produces in `lib/chart.ts`: `zeroOffset(balances: number[]): number` (gradient stop 0..1 where the series crosses zero, measured from the top: `max / (max - min)`; all non-negative → `1`, all non-positive → `0`, empty → `1`); `lowPoint(f: Forecast): { date: string; balance_cents: number } | null` (from `f.shortfall` via `lowest_day`, else the minimum `points` entry; `null` when `points` is empty).
- Produces: `ForecastChart({ forecast }: { forecast: Forecast })`, a Recharts `AreaChart` with a dashed zero `ReferenceLine`, a labelled `ReferenceDot` at the low point, compact-euro y-axis (`formatMoneyCompact`), a tooltip with date and balance, areas below zero in `--danger`, line in `--accent`, `draw-in` on the line, and `role="img"` with the existing `aria-label` pattern.
- Consumes: `Forecast`, `formatMoneyCompact`, `formatDate`, Task 2 `Card`.

- [ ] **Step 1: Failing tests** in `lib/chart.test.ts`: `zeroOffset([5,10,20]) === 1`; `zeroOffset([-5,-10]) === 0`; `zeroOffset([10,-10]) === 0.5`; `zeroOffset([]) === 1`; `lowPoint` returns the `shortfall.lowest_day` point when a shortfall exists; returns the minimum point when `shortfall` is `null`; returns `null` for empty `points`. In `tests/motion.test.ts` add `it("shows the chart line fully when motion is reduced")`: the reduce block must contain a `.draw-in` rule that resets `stroke-dashoffset` to `0` (assert with a regex on the block).
- [ ] **Step 2: Run both.** Expected: FAIL.
- [ ] **Step 3: Implement `lib/chart.ts`,** add the `.draw-in` reset to the reduced-motion block in `globals.css`, then the chart component. When `points` is empty render a Card message "No forecast data yet." instead of the chart.
- [ ] **Step 4: Rebuild the page:** `PageHeader` with `forecast.headline` and available balance; chart Card; the "Get €X now" action as a primary Button inside a prominent Card when `forecast.shortfall`; keep `OfferPanel` content and the server actions; events table with caption "Money expected in and out". Fill the `/design` "states" section with three fixture forecasts (never negative, always negative, empty).
- [ ] **Step 5: Run `npx vitest run lib/chart.test.ts tests/motion.test.ts`** (PASS), `npm run build`; then open `/forecast` against the Docker API and `/design` and confirm the three fixture states render without console errors.

### Task 7: Reminders and Chat

**Files:**
- Modify: `web/app/(app)/reminders/page.tsx`, `web/app/(app)/chat/page.tsx`, `web/components/ScrollToEnd.tsx` (only if styling requires)

**Interfaces:**
- Consumes: `PageHeader`, `Card`, `Table`, `Badge`, `Input`, `Button`, `ChatCard`, `ChatMessage`. Server actions (`sendChat`, etc.) and the `ScrollToEnd` behaviour are unchanged.

- [ ] **Step 1: Reminders:** three Cards, "How reminders go out" (rules), "Email templates", "Emails sent" (table with status Badge). Titles unchanged apart from styling.
- [ ] **Step 2: Chat:** messages as bubbles (owner right, assistant left, system centred), cards for invoice/offer/debtors/warning under messages using the new Card, example chips `["Invoice Kostas €120", "Show outstanding receivables", "Will I go negative?"]`, sticky composer. Keep the `aria-label="Conversation"` list.
- [ ] **Step 3: Verify:** `npm run lint`, `npm run build`; send each example in the running app and confirm replies render. If time is short, restyle with tokens only (spec's cut rule).

### Task 8: `/login` and `/account`

**Files:**
- Create: `web/app/login/page.tsx`, `web/app/(app)/account/page.tsx`, `web/lib/account.ts`, `web/lib/account.test.ts`

**Interfaces:**
- Produces: `initials(name: string): string` in `lib/account.ts`: first letters of the first two words, upper-case; one word → one letter; empty or whitespace → `"?"`.
- Consumes: `getApi().getMe(): Promise<User>`, `apiErrorView`, `Card`, `Button`, `ACCOUNT_HREF`.

- [ ] **Step 1: Failing tests** in `lib/account.test.ts`: `initials("Andreas Kostas") === "AK"`, `initials("andreas") === "A"`, `initials("  ") === "?"`, `initials("Maria Anna Petrou") === "MA"`.
- [ ] **Step 2: Run it.** Expected: FAIL.
- [ ] **Step 3: Implement `initials`.** `/login`: static page outside the `(app)` group (no shell, no data fetching, `metadata.title` "Sign in · Kerdos"): brand, one line of value text, primary link "Continue to the demo" to `/`, a disabled "Continue with Google" Button with the visible note "Google sign-in isn't enabled on this server yet." `/account`: `PageHeader` "Account", profile Card from `getMe()` (initials avatar, name, email, role), link "Sign-in options" to `/login`; wrap in `Suspense` with `PageFallback` and `apiErrorView` like the other pages.
- [ ] **Step 4: Run `npx vitest run lib/account.test.ts`** (PASS). Stop the API container, load `/login` (must render) and `/account` (must show `ErrorPanel` inside the shell), then start it again.

### Task 9: Copy pass, DESIGN.md, cleanup, full verification

**Files:**
- Create: `web/DESIGN.md`, `web/tests/copy.test.ts`
- Modify: `backend/internal/app/chat.go:239`, any file the copy test flags
- Delete: `web/components/ui/Paper.tsx`, `Surface.tsx`, `StatusStamp.tsx`, `web/app/page.module.css`, unused `web/public/*.svg`, if nothing imports them

**Interfaces:**
- Produces: `tests/copy.test.ts` reading every `.ts`/`.tsx` under `app/` and `components/`.

- [ ] **Step 1: Failing test** `tests/copy.test.ts`: no source file contains `/who owes/i`; no file imports `Paper`, `Surface` or `StatusStamp`; no file under `app/` or `components/` references the deleted classes `slab`, `stamp`, `slip` or `shell__` (assert each with a regex over file text).
- [ ] **Step 2: Run it.** Expected: FAIL on whatever remains.
- [ ] **Step 3: Fix what it flags,** delete the dead files, and change the chip in `backend/internal/app/chat.go:239` from `"Who owes me?"` to `"Show outstanding receivables"`. Read through each page's headings, labels and empty states against the copy rules.
- [ ] **Step 4: Write `web/DESIGN.md`:** tokens (value and role), type, component list with intended use, motion rules (durations, easings, reduced motion), copy rules, and the login/account decisions, each with the reason.
- [ ] **Step 5: Run the gates.** In `web/`: `npm test`, `npm run lint`, `npm run build`; all must pass. For the backend run `go test ./internal/app/... ./internal/httpapi/...` from `backend/` (local Go if installed, otherwise `docker run --rm -v "<repo>/backend:/src" -w /src golang:1.23 go test ./internal/app/... ./internal/httpapi/...`); expected PASS.
- [ ] **Step 6: Walk the app** against the Docker API with reduced motion off and then on, at 1280px and 360px: every page, `/login`, `/account`, the chart hover, a long client name and a large amount (use `/design` fixtures if the seed lacks them). Expected: no horizontal page scroll, no console errors, tables scroll inside their wrapper.
- [ ] **Step 7: Hand over.** Run `git status` and `git diff --stat`; propose 2–3 commit messages and wait for the user to choose. Do not commit.
