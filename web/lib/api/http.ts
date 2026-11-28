import { ApiError, type ApiClient } from "./client";
import type {
  Account,
  ApiErrorBody,
  AuthStatus,
  ChatMessage,
  Dashboard,
  Debtor,
  Forecast,
  Invoice,
  Message,
  Offer,
  Invite,
  InviteInfo,
  ReminderPolicy,
  SignedIn,
  Team,
} from "./types";

type Options = {
  baseUrl: string;
  // Cookie header value to forward, for example "kerdos_session=...".
  getCookie?: () => Promise<string | undefined>;
  fetch?: typeof fetch;
};

type Query = Record<string, string | number | undefined>;
const enc = encodeURIComponent;

export function createHttpApi(opts: Options): ApiClient {
  const doFetch = opts.fetch ?? fetch;

  async function call<T>(
    method: "GET" | "POST" | "PATCH" | "DELETE",
    path: string,
    init: { query?: Query; body?: unknown } = {},
  ): Promise<T> {
    const url = new URL(`${opts.baseUrl.replace(/\/+$/, "")}/api${path}`);
    for (const [key, value] of Object.entries(init.query ?? {})) {
      if (value !== undefined && value !== "") url.searchParams.set(key, String(value));
    }
    // The token header asks the backend to include the session token when it signs someone in,
    // so the web app can keep it in its own cookie.
    const headers = new Headers({ accept: "application/json", "x-kerdos-return-token": "1" });
    if (init.body !== undefined) headers.set("content-type", "application/json");
    const cookie = await opts.getCookie?.();
    if (cookie) headers.set("cookie", cookie);

    let res: Response;
    try {
      res = await doFetch(url, {
        method,
        headers,
        body: init.body === undefined ? undefined : JSON.stringify(init.body),
        cache: "no-store",
      });
    } catch {
      throw new ApiError(0, "unreachable", "The Kerdos backend is unreachable. Is it running?");
    }
    if (!res.ok) throw await toApiError(res);
    if (res.status === 204) return undefined as T;
    return (await res.json()) as T;
  }

  return {
    getAuthStatus: () => call<AuthStatus>("GET", "/auth/status"),
    signUp: (input) => call<SignedIn>("POST", "/auth/signup", { body: input }),
    logIn: (email, password) => call<SignedIn>("POST", "/auth/login", { body: { email, password } }),
    logOut: () => call<void>("POST", "/auth/logout"),
    exchangeLoginCode: (code) => call<SignedIn>("POST", "/auth/exchange", { body: { code } }),
    getInvite: (token) => call<InviteInfo>("GET", `/auth/invites/${enc(token)}`),
    startGoogle: async (returnTo, invite) =>
      (await call<{ url: string }>("POST", "/auth/google/start", { body: { return_to: returnTo, invite } })).url,

    getMe: () => call<Account>("GET", "/me"),
    updateMe: (input) => call<Account>("PATCH", "/me", { body: input }),
    changePassword: (currentPassword, newPassword) =>
      call<void>("POST", "/me/password", { body: { current_password: currentPassword, new_password: newPassword } }),
    linkGoogle: async (returnTo) => (await call<{ url: string }>("POST", "/me/google/link", { body: { return_to: returnTo } })).url,
    unlinkGoogle: () => call<Account>("DELETE", "/me/google"),
    logOutEverywhere: () => call<void>("POST", "/auth/logout-everywhere"),

    getTeam: () => call<Team>("GET", "/team"),
    invite: (email, name) => call<{ invite: Invite; link: string }>("POST", "/team/invites", { body: { email, name } }),
    revokeInvite: (id) => call<void>("DELETE", `/team/invites/${enc(id)}`),
    removeMember: (id) => call<void>("DELETE", `/team/members/${enc(id)}`),

    getDashboard: () => call<Dashboard>("GET", "/dashboard"),

    listChat: async () => (await call<{ messages: ChatMessage[] }>("GET", "/chat")).messages,
    sendChat: async (text) => (await call<{ messages: ChatMessage[] }>("POST", "/chat", { body: { text } })).messages,

    listInvoices: async (filter = {}) =>
      (await call<{ invoices: Invoice[] }>("GET", "/invoices", { query: filter })).invoices,
    remindInvoice: (id) => call<Message>("POST", `/invoices/${enc(id)}/remind`),
    cancelInvoice: (id) => call<Invoice>("POST", `/invoices/${enc(id)}/cancel`),
    listDebtors: async () => (await call<{ debtors: Debtor[] }>("GET", "/debtors")).debtors,

    getForecast: (horizonDays) => call<Forecast>("GET", "/forecast", { query: { horizon: horizonDays } }),
    createOffer: () => call<Offer>("POST", "/financing/offers"),
    getOffer: (id) => call<Offer>("GET", `/financing/offers/${enc(id)}`),
    acceptOffer: (id) => call<{ offer: Offer; forecast: Forecast }>("POST", `/financing/offers/${enc(id)}/accept`),

    listMessages: async (filter = {}) =>
      (await call<{ messages: Message[] }>("GET", "/messages", { query: filter })).messages,
    getReminderSettings: () => call<ReminderPolicy>("GET", "/reminders/settings"),
  };
}

async function toApiError(res: Response): Promise<ApiError> {
  try {
    const body = (await res.json()) as Partial<ApiErrorBody>;
    if (body.error?.code && body.error.message) return new ApiError(res.status, body.error.code, body.error.message);
  } catch {
    // Not JSON: fall through to the generic error.
  }
  return new ApiError(res.status, "unexpected", `The backend returned ${res.status}.`);
}
