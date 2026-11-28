# Kerdos design system

A dark navy control room: hairline borders, one bright accent, tabular numerals. Motion only answers something the user did or orients them on arrival. Each rule below carries its reason so new screens stay on-system.

## Tokens

Defined once as plain hex in `app/globals.css` (`:root`) and mapped to Tailwind with `@theme inline`. `tests/contrast.test.ts` reads these values, so change them there and keep text pairs at WCAG AA (4.5:1).

| Token | Value | Role |
|---|---|---|
| `--background` | `#0b1220` | Page. |
| `--surface` | `#111a2e` | Cards, sidebar, tables. |
| `--surface-raised` | `#18233b` | Hover, active nav, popovers. |
| `--border` | `#25324d` | Hairlines. Structure comes from borders, not shadows. |
| `--foreground` / `--muted` | `#e6ecf7` / `#93a4c3` | Text and secondary text. |
| `--accent` / `--accent-foreground` | `#2dd4bf` / `#04201c` | Primary action, links, chart line. Used sparingly so it always means "act here". |
| `--success` `--warning` `--danger` `--info` | green, amber, red, blue | Status only. Never decoration. |

Tailwind names: `bg-background`, `bg-surface`, `text-muted`, `border-border`, `bg-accent`, and so on.

## Type

One sans family (Inter, latin, greek and cyrillic subsets) so Cypriot and Greek client names render correctly. Money, dates and counts use tabular numerals (`tabular-nums` / `.tabular`) so columns line up. Titles are sentence case, semibold, tight tracking.

## Components (`components/ui/`)

| Component | Use |
|---|---|
| `Button` | `primary` for the one main action per area, `quiet` for the rest, `danger` for destructive. `asChild` to render a link. |
| `Card` | Every grouped block. One level of nesting at most. |
| `Badge`, `StatusBadge` | Status chips. `StatusBadge` takes a `DisplayStatus` and uses `lib/status.ts` for label and tone. Give it `key={status}` to replay its entrance when the status changes. |
| `Table` family | Always pass `caption`. The wrapper scrolls sideways so a wide table never widens the page. |
| `Input`, `Select` | Native controls, so forms work without JavaScript. |
| `Tabs`, `Tooltip`, `Dialog` | Radix primitives. `Dialog` doubles as the mobile navigation drawer. |
| `Skeleton` | Loading placeholders (`PageFallback`). |

Page structure: `Shell` (sidebar, top bar, `<main>`) then `PageHeader` then content in a `grid gap-6`. Pages never render their own `<main>`.

`/design` renders every component and the chart's states (never negative, below zero, crossing zero, empty). Use it for visual checks.

## Forecast chart

`components/ForecastChart.tsx` (Recharts, client component). Teal above zero and red below, split exactly at the zero line (`lib/chart.ts` `zeroOffset`). A dashed zero line, round y-axis ticks that always include zero (`niceTicks`), and the lowest point labelled. Gradient ids come from `useId` because SVG ids are page-global. The events table beside it is the accessible data view.

## Motion

CSS only, per the LottieFiles motion-design principles: motion has a purpose, is short, and never blocks content.

- Entrances use `.reveal` (fade up 6px, 240ms, ease-out), staggered 40ms per step with `style={{ "--i": n }}`: header, then cards, then content.
- UI feedback is 150-200ms: hover colour, press scale (0.98), dialog and tooltip open (`tw-animate-css`).
- The chart line draws in once on mount (`.draw-in`, 900ms), the only motion over 500ms.
- `prefers-reduced-motion: reduce` sets animation and transition durations to 0.01ms and shows the chart line fully. `tests/motion.test.ts` guards this.

## Copy

- Noun-based titles: "Outstanding receivables" rather than a question.
- No first-person questions in UI labels. Never "Who owes me?".
- Terms: invoice, client, receivable. The brand is "Kerdos".
- `tests/copy.test.ts` fails on "Who owes" in `app/` and `components/`.

## Login and account

- `/login` is a real page but does not authenticate: the backend only has dev auth (`KERDOS_AUTH=dev`) and no Google sign-in endpoint. It offers "Continue to the demo" and a disabled "Continue with Google" with the note that it is not enabled on this server yet. It shows no fake sign-in.
- `/account` shows name, email and role from `getMe()`. There is no sign-out or edit control because the backend has none. It links to `/login`.
