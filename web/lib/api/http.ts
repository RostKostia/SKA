import { ApiError, type ApiClient } from "./client";
import type {
  ApiErrorBody,
  ChatMessage,
  Dashboard,
  Debtor,
  Forecast,
  Invoice,
  Message,
  Offer,
  ReminderPolicy,
  User,
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

  async function call<T>(method: "GET" | "POST", path: string, init: { query?: Query; body?: unknown } = {}): Promise<T> {
    const url = new URL(`${opts.baseUrl.replace(/\/+$/, "")}/api${path}`);
    for (const [key, value] of Object.entries(init.query ?? {})) {
      if (value !== undefined && value !== "") url.searchParams.set(key, String(value));
    }
    const headers = new Headers({ accept: "application/json" });
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
    return (await res.json()) as T;
  }

  return {
    getMe: () => call<User>("GET", "/me"),
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
