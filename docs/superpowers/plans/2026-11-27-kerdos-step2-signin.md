# Kerdos Step 2: Google Sign-in UI and Session Handling Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans (inline). Steps use checkbox syntax.

**Goal:** A sign-in screen with "Sign in with Google", sign-out, and 401 handling, working in mock mode (a simulated sign-in) and in http mode (the Go backend does OAuth).

**Architecture:** The backend owns OAuth, tokens, refresh, revocation and the httpOnly `kerdos_session` cookie. The frontend only sends the browser to the backend's sign-in URL, forwards the cookie, and clears it on sign-out. Mock mode reuses the same cookie name so the flow looks identical.

**Tech Stack:** as step 1 (Next.js App Router, TypeScript, Vitest).

**Spec:** `docs/superpowers/specs/2026-11-27-kerdos-frontend-design.md` (feature 1), `api/openapi.yaml` (`getSession`, `startGoogleSignIn`, `logout`).

## Global Constraints

- Same as step 1. Additionally: the browser never sees a Google token; the only auth state in the frontend is the `kerdos_session` cookie (httpOnly, SameSite=Lax, path `/`).
- Initial sign-in asks only for identity (name, email). Gmail and Sheets access is requested later, when the reminders and Sheets features need it (contract proposal `scope` query parameter on `startGoogleSignIn`).
- Roles: only `owner` exists (team data model). No role-based UI yet.
- Git: no commits without the user's chosen message (deferred to the end).

## Review Focus

1. Signed-out visit to `/` lands on the sign-in screen, not an error. Verified in the browser (Task 5).
2. Sign-out when the backend is unreachable still clears the local cookie and leaves the user signed out. Pinned in Task 3 (action clears the cookie even if `logout()` rejects).
3. The mock sign-in route must not exist in http mode. Pinned in Task 2.
4. A 401 from any page returns to `/signin`, not Google directly (no redirect loop on a rejected cookie). Task 4.

---

### Task 1: Contract scope parameter and `logout` in the client

**Files:** Modify `api/openapi.yaml`, `web/lib/api/client.ts`, `web/lib/api/http.ts`, `web/lib/api/mock.ts`, `web/lib/api/index.ts`, `web/lib/api/config.ts`. Test: `web/lib/api/http.test.ts`, `web/lib/api/mock.test.ts`, `web/lib/api/config.test.ts`.

**Interfaces:**
- Produces: `ApiClient.logout(): Promise<void>`; `createMockApi(opts?: { today?: string; isSignedIn?: () => Promise<boolean> })` whose `getSession`/`listInvoices` reject with `ApiError(401, "unauthenticated", ...)` when `isSignedIn()` resolves false; `signInUrl(env): string` (`/mock-signin` in mock mode, `/backend/v1/auth/google/start` in http mode); `SESSION_COOKIE` exported from `config.ts`.

- [ ] **Step 1: Write failing tests.** `http.test.ts`: `logout()` sends POST to `/v1/auth/logout` with the cookie and resolves on a 204; a 401 rejects with `ApiError`. `mock.test.ts`: with `isSignedIn: async () => false`, `getSession()` and `listInvoices()` reject with `status 401, code "unauthenticated"`; with `true`, `getSession()` resolves. `config.test.ts`: `signInUrl({})` is `/mock-signin`; `signInUrl({KERDOS_API_MODE:"http",KERDOS_API_URL:"http://go:8080"})` is `/backend/v1/auth/google/start`.
- [ ] **Step 2:** Run `npx vitest run lib/api`. Expected: FAIL.
- [ ] **Step 3:** Implement. Add a `scope` query parameter (`gmail` or `sheets`, `x-proposed`) to `startGoogleSignIn` in `openapi.yaml` and run `npm run gen:api`. In `http.ts` add a `post` helper that tolerates an empty body. `index.ts` passes a cookie-based `isSignedIn` to the mock. Replace the `SIGN_IN_URL` constant with `signInUrl` and export `SESSION_COOKIE = "kerdos_session"`.
- [ ] **Step 4:** Run `npx vitest run && npx tsc --noEmit`. Expected: PASS.

### Task 2: Mock sign-in route

**Files:** Create `web/app/mock-signin/route.ts`. Test: `web/app/mock-signin/route.test.ts`.

**Interfaces:** Consumes `resolveApiConfig`, `SESSION_COOKIE`. Produces `GET(request: NextRequest): NextResponse`.

- [ ] **Step 1: Write failing tests.** In mock mode, `GET` returns a redirect to `/` that sets `kerdos_session` (httpOnly, SameSite lax, path `/`). In http mode it returns 404 and sets no cookie.
- [ ] **Step 2:** Run it. Expected: FAIL (module missing).
- [ ] **Step 3:** Implement with `NextResponse.redirect` and `response.cookies.set`.
- [ ] **Step 4:** Run it. Expected: PASS.

### Task 3: Sign-out action

**Files:** Create `web/app/(app)/actions.ts`. Test: `web/app/(app)/actions.test.ts`.

**Interfaces:** Produces `signOut(): Promise<never>`, a server action that calls `logout()` (ignoring `ApiError`), deletes the session cookie, then `redirect("/signin")`. Written with injected dependencies (`runSignOut(deps)`) so it can be tested without Next's request scope.

- [ ] **Step 1: Write failing tests.** `runSignOut` calls `logout`, deletes the cookie and redirects to `/signin`; when `logout` rejects with `ApiError(0, "unreachable", ...)` it still deletes the cookie and redirects.
- [ ] **Step 2:** Run. Expected: FAIL.
- [ ] **Step 3:** Implement `runSignOut` and the thin `signOut` wrapper that supplies `getApi()`, `cookies()` and `redirect`.
- [ ] **Step 4:** Run. Expected: PASS.

### Task 4: Sign-in page, sign-out key, 401 handling

**Files:** Create `web/app/signin/page.tsx`. Modify `web/components/ErrorPanel.tsx`, `web/components/Shell.tsx`, `web/app/globals.css` only if needed.

- [ ] **Step 1:** `/signin` (outside the app shell): the Kerdos mark, one sentence on what signing in shares ("Signing in shares your name and email address. Kerdos asks for Gmail and Sheets access later, only when you turn on reminders or Sheets sync."), and a primary key-styled link "Sign in with Google" to `signInUrl(process.env)`. Sentence case, no arrows.
- [ ] **Step 2:** `apiErrorView`: a 401 redirects to `/signin` (not straight to Google).
- [ ] **Step 3:** `Shell`: show the user's name and a "Sign out" key (a form posting `signOut`) in the top bar.
- [ ] **Step 4:** `npm run lint && npx tsc --noEmit && npm run build`. Expected: all pass.

### Task 5: Browser verification

- [ ] Run `npm run dev`. Visit `/` signed out: lands on `/signin`. Click "Sign in with Google": lands on `/` with data. Click "Sign out": lands on `/signin`; visiting `/` again returns to `/signin`. With `KERDOS_API_MODE=http` and a dead URL, `/mock-signin` returns 404 and `/` shows the unreachable panel. Stop only the dev server started here.

## Self-review

Coverage: sign-in screen (Task 4), sign-out (3, 4), session cookie handling (1, 2), 401 handling (4), incremental scopes (1 contract, copy in 4). Names are consistent: `signInUrl`, `SESSION_COOKIE`, `runSignOut`, `signOut`, `isSignedIn`.
