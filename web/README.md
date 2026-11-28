# Kerdos web

Next.js frontend for Kerdos. It shows what the Go backend (`../backend`) returns and sends the owner's actions to it; it holds no business logic and no database access. The backend's endpoint table in [`../backend/README.md`](../backend/README.md) is the contract, and `lib/api/types.ts` mirrors its JSON.

## Run

The backend must be running first (see [Quick start](../project-description.md#quick-start) in the project description). Then:

```sh
npm install
npm run dev      # http://localhost:3000, talks to http://127.0.0.1:8080
npm test         # unit tests (Vitest)
npm run lint
npm run build && npm start
```

| Variable | Default | Meaning |
|---|---|---|
| `KERDOS_API_URL` | `http://127.0.0.1:8080` | Where the Go backend listens |

## Pages

| Page | What it does |
|---|---|
| `/` | Who owes you, the forecast warning, one slip per open invoice |
| `/chat` | "Invoice Kostas €120" is sent at once; the backend asks for anything missing. Warning and offer cards: "Get €800 now", then Accept |
| `/invoices` | "From the company" and "To the company" tabs, status filter, search, sort, Remind now, Cancel, Export CSV |
| `/debtors` | Who owes what (financed invoices are left out), Remind now |
| `/forecast` | Balance chart and headline from the backend, what it counts, "Get €800 now" with the fee shown before you accept |
| `/reminders` | How automatic reminders go out, the email templates, and every email sent |
| `/design` | The design kit |

## Not built yet (the backend has no endpoint for these)

Google sign-in and sign-out (the backend runs with `KERDOS_AUTH=dev`), Sheets sync, Excel export, budgets and category forecasts, editing reminder rules and templates, writing a reminder's text yourself, and the client's pay page (`/pay/{token}`, the backend's `GET /api/pay/{token}` is ready for it).
