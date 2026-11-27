# Kumbares clickable mockup: design (short)

Status: approved by the user in conversation on 2026-11-27. Not committed.

## Intent and why
A clickable desktop mockup of the Kumbares app, built from `KUMBARES_BRIEF.md`. It is the next stage after the brief: it shows what the MVP looks like and how it behaves, and serves as the visual spec for the hackathon build. It is not a pitch and not a deck.

## Goals
- Cover the whole MVP: invoice by chat, invoice list, reminders (with escalating tone), debtors summary, cash forecast chart, action plan with the "get money now" financing button.
- Show the 3-step demo path end to end: "Invoice Kostas €120", skip time and an automatic reminder, then the cash warning and the financing button.
- Include the client's invoice and pay page (phone-sized), standing in for the second phone in the demo.
- Keep all sample data in one file, mirroring the brief's "mock adapter behind the same interface" rule.

## Non-goals
- No real API calls (BoC, Jinius, Claude). The financing button is simulated.
- No WhatsApp, till cash, language switching, reliability score or what-if chat (nice-to-haves left out).
- No pitch, deck or Figma file.

## Anti-goals
- Implying anything is connected to real services or real money.
- Copying Bank of Cyprus branding.

## Constraints
- Desktop web dashboard layout, hi-fi, clickable HTML, English UI.
- No build step, no dependencies, opens by double-click.
- All names, amounts and dates are labelled as sample data.

## Approaches considered
- A. One self-contained `index.html`: fastest, but unwieldy at this scope and hard to reuse.
- B. Small static folder, no build step (chosen): editable, data separated from UI, no installs.
- C. Vite and React scaffold: closest to a real MVP, but adds dependencies and build time. Ruled out.

## Design
Location: `mockup/` with `index.html`, `styles.css`, `app.js`, `data.js`.

Layout: left nav (Chat, Invoices, Debtors, Forecast, Reminders), central chat, right panel with the forecast chart, who-owes-what and the action plan. Top bar has a "Skip time" button and a "Client view" button.

Screens: Chat, Invoices, Debtors, Forecast, Reminders settings, Client invoice page (phone frame).

Look (presenter's call): calm blue-green fintech palette, one accent, one typeface pair, WCAG AA contrast, body text at least 16 px.

## Left to the builder
Exact copy, sample data values, chart drawing (inline SVG), and how the time skip steps through the demo state.
