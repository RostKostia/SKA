# Kerdos UI rebuild: design

Status: draft for the user's approval, 2026-11-28. Not committed.

## Intent and why
The current web app (limestone counter, paper slips, rubber stamps) reads as bland, and the user lists concrete faults: a poor forecast chart, a poor page layout, no login page, no account page, and unprofessional product copy ("Who owes me?"). Rebuild the UI layer of `web/` as a modest, distinctive, professional **dark navy "control room"** (direction B, chosen by the user), with purposeful motion. The audience is a small-business owner (and the hackathon judges) looking at invoices, debtors, reminders and a cash forecast.

## Goals
- A new visual system on Tailwind CSS with shadcn/ui-style components, replacing `globals.css` and `components/ui/*`.
- All 6 app pages (Overview, Chat, Invoices, Debtors, Forecast, Reminders) rebuilt on the new system with a clear layout: persistent sidebar, page header, content grid.
- A `/login` page and an `/account` page.
- A forecast chart that is legible and honest: balance line/area, zero line, labelled low point, hover values, shortfall highlighted.
- A copy pass so every label reads as professional product language. "Who owes you" becomes "Outstanding receivables"; the chat example "Who owes me?" becomes "Show outstanding receivables".
- Motion that follows the LottieFiles motion-design principles: short, purposeful, off under `prefers-reduced-motion`.

## Non-goals
- No change to the API client, data fetching, server actions, proxy routes or the Go backend, except one copy string (see "My calls"). Server Components stay server-rendered.
- No new product features, no light theme, no theme switcher, no language switching.
- No real Google OAuth, sign-out or session handling: the backend only has `KERDOS_AUTH=dev` today.
- No Storybook, no visual-regression tooling.

## Anti-goals
- Pixel-copying any brand from the VoltAgent catalog. It is used for how a `DESIGN.md` is structured, not for a company's identity.
- Motion that decorates without informing, or that hides content until it finishes.
- Mock data presented as real, or "Soon" dead links.
- Losing existing accessibility: focus rings, table captions, `aria-label`s, contrast.

## Constraints
- Approved new dependencies, and only these: `tailwindcss`, `@tailwindcss/postcss` (4.3.5), `radix-ui` (1.8.1), `class-variance-authority` (0.7.1), `clsx` (2.1.1), `tailwind-merge` (3.7.1), `lucide-react` (1.83.0), `recharts` (3.11.2), `tw-animate-css` (1.4.0). No motion library; motion is CSS.
- Next.js 16.4.6 differs from common knowledge: `web/AGENTS.md` requires reading the relevant guide in `node_modules/next/dist/docs/` before writing Next-specific code.
- Existing tests keep passing or are updated to the new token source: `tests/contrast.test.ts` (reads tokens from `globals.css`), `tests/motion.test.ts` (reads the reduced-motion block), plus the `lib/` and route tests.
- Public repo: no secrets; env-only config. No commits unless the user asks.
- Deadline: hackathon ends 29 November.

## Approaches considered
- **1. Reskin in place** (tokens plus four UI pieces). Smallest and lowest risk, but keeps the current structure the user finds weak.
- **2. Rebuild the UI layer by hand** (new components and CSS). Cleaner, but about 1,000 lines of hand-written CSS and components, with no shared primitives for dialogs, menus or tooltips.
- **3. Adopt a UI and chart library.** Standard primitives and a real chart, at the cost of dependencies and a client-side chart.
- **Chosen by the user: 2 + 3 combined**: rebuild the UI layer on Tailwind, Radix-based shadcn/ui components and Recharts.

## Design

**Visual system (direction B).** Navy surfaces, hairline borders, one bright accent, semantic status colours. Tokens live in `app/globals.css` as Tailwind v4 `@theme` variables (single source for CSS and tests). Starting values, to be tuned against the contrast test:

| Token | Role | Start value |
|---|---|---|
| `--background` | page | `#0b1220` |
| `--surface` | cards, sidebar | `#111a2e` |
| `--surface-raised` | popovers, hover | `#18233b` |
| `--border` | hairlines | `#25324d` |
| `--foreground` / `--muted` | text | `#e6ecf7` / `#93a4c3` |
| `--accent` | primary action, links, chart line | `#2dd4bf` |
| `--success` / `--warning` / `--danger` / `--info` | status | green, amber, red, blue |

Type: one sans family for UI (loaded with `next/font`, replacing Commissioner and Source Serif), tabular numerals for money everywhere.

**Shell.** Fixed left sidebar (brand, nav with Lucide icons, account entry at the bottom) and a page header (title, optional description, primary action). Under 768px the sidebar becomes a top bar with a menu button (Radix dialog). The existing `Shell`, `NavLinks`, `FlashBanner`, `ErrorPanel`, `PageFallback` keep their roles and are restyled.

**Components** (copied-in shadcn/ui style, in `components/ui/`): Button, Card, Badge (replaces `StatusStamp`; same `DisplayStatus` labels), Table, Input, Tabs, Tooltip, Dialog, Skeleton. `Paper`, `Surface` and the stamp CSS are removed.

**Pages.**
- **Overview:** headline KPI row (total outstanding, overdue, next expected payment), then a list of open invoices with Badge status.
- **Invoices:** Tabs for direction, a search/filter toolbar, a Table with row actions. Unchanged data and server actions.
- **Debtors:** page title "Outstanding receivables"; a table grouped by client with amount and age.
- **Forecast:** header with the headline and available balance, the chart as the hero card, the "get money now" offer as a prominent card when there is a shortfall, then the events table.
- **Reminders:** rules, templates and sent log as three Cards.
- **Chat:** message log, example prompts as chips, sticky composer.
- **`/login`:** centred card outside the app shell: brand, one line of value text, a primary "Continue to the demo" link to `/`, and below it a disabled "Continue with Google" button with the visible note "Google sign-in isn't enabled on this server yet". The backend has no Google sign-in endpoint (the only `authorize_url` handler is the bank connection, `server.go:412`), so the page is static, needs no auth-mode detection, and does not pretend to authenticate. If the backend rejects requests, `/` already shows `ErrorPanel`.
- **`/account`:** profile card from `getMe()` (initials avatar, name, email, role). No sign-out control, because the backend has none. A "Sign-in options" link goes to `/login`, and the sidebar's account entry goes to `/account`, so `/login` is reachable in the UI.

**Forecast chart.** A client component (`"use client"`) built with Recharts `AreaChart`: balance area/line, dashed zero reference line, a labelled marker at the lowest point, areas below zero filled with the danger colour, a tooltip with date and balance, y-axis in compact euro. Receives the existing `Forecast` object as props; no new data calls. Provide a text alternative (`aria-label` with the headline, as today) and keep the events table as the accessible data view.

**Motion** (CSS only, `tw-animate-css` plus a few `@keyframes`), per the LottieFiles principles: 150–250 ms for UI feedback, ease-out for entrances, nothing over 500 ms except the chart draw-in.
- Page content fades up with a 40 ms stagger on header, KPI cards, then main content.
- Chart line draws in once on mount.
- Badge status change replays a short scale/colour transition (`key={status}` pattern already used).
- Buttons and rows get hover/press feedback; dialogs and menus use the Radix open/close states.
- A single `@media (prefers-reduced-motion: reduce)` block sets animation and transition durations to `0.01ms`, preserving the `motion.test.ts` contract.

**Copy.** A pass over every page title, label, empty state and example prompt. Rules: noun-based titles ("Outstanding receivables", "Cash forecast"), no first-person questions in UI labels, consistent terms (invoice, client, receivable). The brand name stays "Kerdos". The product brief and README wording is out of scope unless the user asks.

**A `DESIGN.md`.** Add `web/DESIGN.md` documenting the tokens, components, motion rules and copy rules, in the VoltAgent `DESIGN.md` structure (token, rule and rationale together), so future screens stay on-system. The existing `/design` demo route is rebuilt to show the new components and is the visual QA page.

**Testing and proof.** `npm test` (token contrast and reduced-motion tests updated to read the new tokens), `npm run lint`, `npm run build`, then run the app against the Docker API (`--profile app`, seeded demo data; there is no mock mode in `web/lib/api`) and walk each page, including `/login` and `/account`, at desktop and phone widths, with reduced motion on. States the seed doesn't produce (empty lists, shortfall, error) are checked on the `/design` page with fixture props, not by changing the API client.

## Order of work
1. Install the approved dependencies, set up Tailwind, tokens, fonts, `cn` helper.
2. Components, shell, `/design` page.
3. Pages in order: Overview, Invoices, Debtors, Forecast (chart), Reminders, Chat.
4. `/login`, `/account`.
5. Copy pass, `DESIGN.md`, remove dead CSS and components, tests and build.

## Left to the builder
- Exact colour values and type family, subject to the contrast test (WCAG AA for text) and a restrained look.
- Component file layout inside `components/ui/` and which shadcn/ui components are needed beyond the list.
- Exact KPI definitions on Overview, derived from data the API already returns.
- Whether `/account` also shows read-only service status, only if the API client already exposes it.

## My calls (change any of these)
- Dark theme only; no light mode.
- Motion stays CSS-only; no animation library.
- `/login` is a real page but does not authenticate beyond what the backend supports; no route protection added.
- `/account` has no sign-out and no edit controls.
- Hand-written SVG chart and the old stamp, slip and paper styles are deleted, not kept as options.
- `web/DESIGN.md` is added as the design reference.
- The backend's own chat suggestion chip "Who owes me?" (`backend/internal/app/chat.go:239`; `demo_test.go:262` sends the same text and still passes, since the parser matches either wording) is changed to "Show outstanding receivables"; the parser already matches "outstanding". This is the only Go change.
- If time runs short, Chat and Reminders get the new tokens and shell only, not a layout rebuild. The chart, shell, Overview, Invoices, Debtors, `/login` and `/account` come first.
