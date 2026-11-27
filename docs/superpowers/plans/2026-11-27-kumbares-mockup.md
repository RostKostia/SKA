# Kumbares Clickable Mockup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a clickable desktop HTML mockup of the whole Kumbares MVP, including the 3-step demo and the client's invoice page.

**Architecture:** Static folder `mockup/`, no build step, no dependencies. Pure functions in `logic.js` (tested with Node's built-in `node:test`), sample data in `data.js`, rendering and events in `app.js`. State is one plain object, replaced (never mutated) on each action, then re-rendered.

**Tech Stack:** HTML, CSS, vanilla JavaScript (browser globals, plus `module.exports` in `logic.js` for tests), inline SVG for the chart.

**Spec:** `docs/superpowers/specs/2026-11-27-kumbares-mockup-design.md` (source brief: `KUMBARES_BRIEF.md`)

## Global Constraints

- Desktop web dashboard, hi-fi, English UI, opens by double-click on `mockup/index.html`.
- No build step and no dependencies (Node's built-in test runner only, for `logic.js`).
- No real API calls. The financing button is simulated.
- Out of scope: WhatsApp, till cash, language switching, reliability score, what-if chat.
- All names, amounts and dates are sample data and labelled "Sample data" in the UI.
- Look: calm blue-green fintech palette, one accent colour, one typeface pair, no copy of Bank of Cyprus branding.
- Body text at least 16 px, contrast at least WCAG AA, no meaning carried by colour alone.
- Do not commit. The user asked for commits only on request.
- Presenter's call, to flag to the user: the brief's demo offers "€800 against Kostas's invoice" but Kostas's invoice is €120. The mockup offers financing against the pool of unpaid invoices (Kostas €120, Elena's Bakery €450, Andreas Electric €380 = €950).

## Review Focus

1. Chat input is empty or whitespace: nothing is sent and no card is created.
2. Invoice command with no amount (`Invoice Kostas`) or no client: Kumbares asks a clarifying question instead of creating an invoice.
3. Amounts written as `€40.50`, `40,50`, or `€1,200`: parsed to the right number.
4. "Skip time" clicked repeatedly: reminders are not duplicated, and tone stops escalating at `final`.
5. "Get €800 now" clicked twice, or when the forecast shows no shortfall: the balance is not credited twice and the button is not offered.

---

## File Structure

- `mockup/data.js`: sample state. Exposes `window.KUMBARES_DATA` (`module.exports` too).
- `mockup/logic.js`: pure functions, no DOM.
- `mockup/tests/logic.test.js`: `node:test` tests for `logic.js`.
- `mockup/index.html`: page shell, left nav, view containers, top bar.
- `mockup/styles.css`: design tokens (CSS variables) and layout.
- `mockup/app.js`: state, rendering, navigation, chat, time skip, client view.

State shape (defined in Task 1, used by all later tasks):

```
State = {
  day: number,                      // 0 = today
  balance: number,
  reminderEveryDays: number,        // default 3
  invoices: Invoice[],
  inflows:  {day:number, label:string, amount:number}[],   // expected, non-invoice
  outflows: {day:number, label:string, amount:number}[],
  financed: boolean
}
Invoice = { id:string, client:string, amount:number, description:string,
            sentDay:number, dueDay:number, status:"unpaid"|"paid"|"financed",
            reminders:{day:number, tone:"polite"|"firm"|"final"}[] }
```

---

### Task 1: Sample data and logic, tested

**Files:**
- Create: `mockup/data.js`, `mockup/logic.js`, `mockup/tests/logic.test.js`

**Interfaces:**
- Produces, in `logic.js`:
  - `parseInvoiceCommand(text: string): {client: string, amount: number, description: string} | null`. `"Invoice Kostas €120"` gives `{client:"Kostas", amount:120, description:"Services"}`. `"Invoice Kostas, 3 hours at €40"` gives `{client:"Kostas", amount:120, description:"3 hours at €40"}`. Returns `null` when the client or amount is missing.
  - `reminderTone(reminderNumber: number): "polite" | "firm" | "final"`. 1 is polite, 2 is firm, 3 or more is final.
  - `forecast(state: State, horizonDays?: number): {day: number, balance: number}[]`. Default horizon 30. Point `day` is relative to `state.day` (0 = today), 0 to horizon inclusive. Applies `inflows` and `outflows` whose absolute `day` is after `state.day`, on their day.
  - `shortfall(points): {firstDay: number, lowestDay: number, amount: number} | null`. `amount` is the lowest balance as a positive number. `null` when the balance never goes below 0.
  - `advanceDays(state: State, n: number): State`. Steps one day at a time. An unpaid invoice gets reminder k on day `sentDay + k * reminderEveryDays`, once, with `reminderTone(k)`. Inflows and outflows whose day is reached are applied to `balance` and removed from the lists.
  - `applyFinancing(state: State, amount: number): State`. Marks unpaid invoices `financed`, oldest first, until their sum is at least `amount`. Adds `amount` to `balance` and sets `financed: true`. Returns the same state unchanged if `financed` is already true or unpaid invoices total less than `amount`.
- Produces, in `data.js`: `KUMBARES_DATA: State` with `day: 0`, `balance: 1000`, `reminderEveryDays: 3`, inflow `{day:2, label:"Maria Savva, deposit (sample)", amount:300}`, outflows `{day:4, label:"Shop rent", amount:1500}` and `{day:12, label:"Insurance", amount:600}`, `financed: false`, and unpaid invoices for Elena's Bakery (€450) and Andreas Electric (€380), both with `sentDay: -10`, plus one paid invoice. Kostas's invoice is not in the data. The chat creates it.

- [ ] **Step 1: Write the failing tests** in `mockup/tests/logic.test.js` with `node:test` and `node:assert`. Tests:
  - `parseInvoiceCommand` returns `{client:"Kostas", amount:120}` for `"Invoice Kostas €120"`, amount 120 for `"Invoice Kostas, 3 hours at €40"`, amount 40.5 for `"Invoice Kostas €40.50"`, 40.5 for `"Invoice Kostas 40,50"`, 1200 for `"Invoice Kostas €1,200"`, and `null` for `"Invoice Kostas"`, `"Invoice €120"` and `""`.
  - `reminderTone` gives polite, firm, final, final for 1, 2, 3, 7.
  - `shortfall(forecast(KUMBARES_DATA))` equals `{firstDay:4, lowestDay:12, amount:800}`. It is `null` for a state with `balance: 5000`.
  - `advanceDays(state, 3)` gives each unpaid invoice with `sentDay: 0` exactly one `polite` reminder. `advanceDays` by 3, then 3, then 3, then 3 gives tones polite, firm, final, final. Calling it with `n = 0` changes nothing. After `advanceDays(KUMBARES_DATA, 3)` the balance is 1300 (the day-2 inflow is applied) and `shortfall(forecast(...))` equals `{firstDay:1, lowestDay:9, amount:800}`.
  - `applyFinancing(state, 800)` raises the balance by 800 and marks invoices `financed`. A second call returns the state unchanged. `applyFinancing(state, 5000)` returns it unchanged.
- [ ] **Step 2: Run** `node --test mockup/tests/` and confirm it fails (module not found).
- [ ] **Step 3: Implement `logic.js` and `data.js`** to the interfaces above. `logic.js` ends with `if (typeof module !== "undefined") module.exports = {...}` and otherwise attaches to `window.KumbaresLogic`. Treat a lone comma followed by one or two digits as a decimal, and a comma followed by three digits as a thousands separator.
- [ ] **Step 4: Run** `node --test mockup/tests/`. Expected: all pass.

---

### Task 2: Shell, styles and the static screens

**Files:**
- Create: `mockup/index.html`, `mockup/styles.css`
- Create: `mockup/app.js` (navigation and the Invoices, Debtors and Reminders views only)

**Interfaces:**
- Consumes: `KUMBARES_DATA`, `KumbaresLogic.reminderTone` (Task 1).
- Produces, in `app.js`:
  - `setState(next: State): void`: replaces the state and calls `render()`.
  - `render(): void`: re-renders the active view and the right panel.
  - `showView(name: "chat"|"invoices"|"debtors"|"forecast"|"reminders"|"client"): void`.
  - Container ids in `index.html`: `#view-chat`, `#view-invoices`, `#view-debtors`, `#view-forecast`, `#view-reminders`, `#view-client`, `#side-panel`, `#btn-skip-time`, `#btn-client-view`.

- [ ] **Step 1: Build the shell.** Left nav with five items (Chat, Invoices, Debtors, Forecast, Reminders), top bar with "Skip time ▸ +3 days" and "Client view ↗", a "Sample data" badge, and a right-hand `#side-panel`. Define colour, type and spacing as CSS variables on `:root`. Nav items are `<button>`s with visible focus styles.
- [ ] **Step 2: Render Invoices.** A table of all invoices with client, amount, sent date, status (text label plus icon, not colour alone) and reminders sent.
- [ ] **Step 3: Render Debtors.** Per client: unpaid total, oldest unpaid age in days, last reminder tone. Sorted by total owed.
- [ ] **Step 4: Render Reminders settings.** Shows the reminder interval (3 days) and the three tone steps with a sample message for each (polite, firm, final). Controls are static.
- [ ] **Step 5: Verify.** Open `mockup/index.html` in a browser. Expected: each nav item switches the view, there are no console errors, and focus is visible when tabbing.

---

### Task 3: Forecast chart and side panel

**Files:**
- Modify: `mockup/app.js`, `mockup/styles.css`

**Interfaces:**
- Consumes: `forecast`, `shortfall` (Task 1), `render`, `setState` (Task 2).
- Produces: `renderForecastChart(points, shortfallInfo, container: HTMLElement): void`. It draws an inline SVG line chart with a zero baseline, the negative area shaded, and a direct label at the lowest point ("Day 12: −€800"). It carries a text description for screen readers. `renderSidePanel()` shows the shortfall warning headline, who-owes-what totals and the action-plan list (remind specific clients, "Get €800 now"). The Forecast view shows a larger chart plus the list of expected inflows and outflows.

- [ ] **Step 1: Implement `renderForecastChart`.** Y axis includes 0. Use the `dataviz` skill for colour and label choices. Shortfall is marked by a pattern or label as well as colour.
- [ ] **Step 2: Implement `renderSidePanel` and the Forecast view.** With sample data the warning reads "In 12 days you'll be €800 negative". When `shortfall` is `null` it reads "No shortfall in the next 30 days" and the financing button is not rendered.
- [ ] **Step 3: Verify.** Open in a browser. Expected: the chart shows the dip at day 4 and the low of −€800 at day 12, and the text description matches.

---

### Task 4: Chat and the demo flow

**Files:**
- Modify: `mockup/app.js`, `mockup/styles.css`

**Interfaces:**
- Consumes: `parseInvoiceCommand`, `advanceDays`, `applyFinancing`, `forecast`, `shortfall` (Task 1), `setState` (Task 2).
- Produces: `handleChatSubmit(text: string): void`, `handleSkipTime(): void`, `handleFinancing(): void`. Chat messages live in a module-level array and render into `#view-chat`.

- [ ] **Step 1: Chat.** A message list and an input. `handleChatSubmit` ignores empty or whitespace text. A valid command adds the invoice (`sentDay` = today, `dueDay` = `sentDay + 14`, id `inv-<n>`), shows an invoice card with "Sent via Jinius with payment link, email copy sent (simulated)". An unparseable command gets Kumbares's reply asking for the missing client or amount.
- [ ] **Step 2: Skip time.** `#btn-skip-time` calls `advanceDays(state, 3)` and adds a chat message for each new reminder ("Reminder 1 sent to Kostas, polite tone"). Repeated clicks continue to work, and tone stops at `final`.
- [ ] **Step 3: Warning and financing.** After the skip, the chat shows "In <lowestDay> days you'll be €800 negative. Get €800 against your unpaid invoices now?" (9 days after one 3-day skip; 12 before any skip) with a button. `handleFinancing` calls `applyFinancing(state, 800)`, shows a confirmation card (simulated, marked "Sample"), and removes the button.
- [ ] **Step 4: Verify the demo path in a browser.** Type "Invoice Kostas €120", click Skip time, click the financing button. Expected: invoice card, then reminder message, then the warning and button, then the balance goes up by €800 and the forecast chart no longer dips below zero. Check that an empty submit and `Invoice Kostas` behave as in Review Focus 1 and 2, and that a second financing click does nothing.

---

### Task 5: Client invoice page and final check

**Files:**
- Modify: `mockup/app.js`, `mockup/styles.css`

**Interfaces:**
- Consumes: `state.invoices`, `showView` (Task 2).
- Produces: `renderClientView(invoice: Invoice | undefined): void`. A phone-sized frame (about 390 px wide) showing the invoice (business name "Sample Business", client, amount, description, due date), a "Pay now" button, and a reminder banner whose wording matches the latest reminder tone. When no invoice exists, it shows "No invoice sent yet".

- [ ] **Step 1: Implement `renderClientView`** for the most recently created invoice, and wire `#btn-client-view`.
- [ ] **Step 2: "Pay now"** marks the invoice `paid` in state (simulated) and shows a "Paid, thank you" confirmation. The Invoices and Debtors views reflect it.
- [ ] **Step 3: Accessibility pass.** Use the `design:accessibility-review` skill on the page. Check contrast, text size, keyboard order, and that statuses are not colour-only. Fix what it finds.
- [ ] **Step 4: Final run.** `node --test mockup/tests/` passes. Walk the full demo in a browser, then confirm there are no console errors and the layout holds at 1280 px and 1440 px wide.
