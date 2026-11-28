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
| `KERDOS_WEB_URL` | the request's host | This app's address in the browser, for Google's return trip. It must match the backend's `KERDOS_PUBLIC_URL` (so open `http://localhost:3000`, not `127.0.0.1:3000`, unless you change both) |

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
| `/login` | Email and password, or Continue with Google. Offers "Create the owner account" while nobody has one |
| `/signup` | The owner's account (first time only), or joining from an invitation link (`?invite=…`): the address is fixed to the invited one |
| `/account` | Name and email, password (or a first password for a Google-only account), link or disconnect Google, sign out, sign out on every device |
| `/team` | Owner only: members, invite by email (the link is also shown to copy), open invitations with Invite again and Cancel, remove a member |
| `/auth/callback` | Where Google sign-in comes back: swaps the one-time code for the session cookie |

## Signing in

With the backend on `KERDOS_AUTH=accounts`, every page needs a session. Signing in stores the backend's session token in this app's own `kerdos_session` cookie (httpOnly, SameSite=Lax), which the server forwards to the backend on every call. A 401 from the backend sends the browser to `/login`. With `KERDOS_AUTH=dev` there is no sign-in: `/login` says so and offers "Continue to the demo", and everything works as the owner.

## Not built yet

The backend has no endpoint yet for: Google Sheets sync, Excel export, budgets and category forecasts, editing reminder rules and templates, and writing a reminder's text yourself.

The backend is ready, but the web app has no page or button yet for:
- the client's pay page: `/pay/{token}`, using `GET` and `POST /api/pay/{token}`
- connecting the bank: `/settings`, using `POST /api/bank/connect` and `POST /api/bank/sync`. BoC consent returns to `/settings?bank=connected` or `?bank=error`
- retrying a failed email: `POST /api/messages/{id}/retry`
