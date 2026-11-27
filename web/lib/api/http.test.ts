import { describe, expect, it } from "vitest";
import { ApiError } from "./client";
import { createHttpApi } from "./http";

type Call = { url: string; method: string; headers: Headers; body: string | null };

function fakeFetch(respond: () => Response | Promise<Response>) {
  const calls: Call[] = [];
  const fetchImpl = (async (input: RequestInfo | URL, init?: RequestInit) => {
    calls.push({
      url: String(input),
      method: init?.method ?? "GET",
      headers: new Headers(init?.headers),
      body: typeof init?.body === "string" ? init.body : null,
    });
    return respond();
  }) as typeof fetch;
  return { calls, fetchImpl };
}

const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { "content-type": "application/json" } });
const api = (fetchImpl: typeof fetch, baseUrl = "http://go:8080") => createHttpApi({ baseUrl, fetch: fetchImpl });

describe("createHttpApi (Go backend routes)", () => {
  it("reads the signed-in user from /api/me", async () => {
    const { calls, fetchImpl } = fakeFetch(() => json({ id: "u1", name: "Owner" }));
    expect(await api(fetchImpl).getMe()).toMatchObject({ name: "Owner" });
    expect(calls[0]).toMatchObject({ url: "http://go:8080/api/me", method: "GET" });
  });

  it("keeps a path prefix in the base URL and tolerates a trailing slash", async () => {
    const { calls, fetchImpl } = fakeFetch(() => json({}));
    await api(fetchImpl, "http://gateway/kerdos/").getMe();
    expect(calls[0].url).toBe("http://gateway/kerdos/api/me");
  });

  it("filters invoices by direction and status and unwraps the list", async () => {
    const { calls, fetchImpl } = fakeFetch(() => json({ invoices: [{ id: "inv_1" }] }));
    expect(await api(fetchImpl).listInvoices({ direction: "payable", status: "overdue" })).toEqual([{ id: "inv_1" }]);
    const url = new URL(calls[0].url);
    expect(url.pathname).toBe("/api/invoices");
    expect(url.searchParams.get("direction")).toBe("payable");
    expect(url.searchParams.get("status")).toBe("overdue");
  });

  it("leaves out filters that are not set", async () => {
    const { calls, fetchImpl } = fakeFetch(() => json({ invoices: [] }));
    await api(fetchImpl).listInvoices();
    expect(calls[0].url).toBe("http://go:8080/api/invoices");
  });

  it("posts a chat message as JSON and unwraps the messages", async () => {
    const { calls, fetchImpl } = fakeFetch(() => json({ messages: [{ id: "m1" }] }));
    expect(await api(fetchImpl).sendChat("Invoice Kostas €120")).toEqual([{ id: "m1" }]);
    expect(calls[0]).toMatchObject({ url: "http://go:8080/api/chat", method: "POST" });
    expect(calls[0].headers.get("content-type")).toBe("application/json");
    expect(JSON.parse(calls[0].body!)).toEqual({ text: "Invoice Kostas €120" });
  });

  it("encodes ids in paths", async () => {
    const { calls, fetchImpl } = fakeFetch(() => json({ offer: {}, forecast: {} }));
    await api(fetchImpl).acceptOffer("a/b c");
    expect(calls[0]).toMatchObject({ url: "http://go:8080/api/financing/offers/a%2Fb%20c/accept", method: "POST" });
  });

  it("forwards the session cookie when there is one", async () => {
    const { calls, fetchImpl } = fakeFetch(() => json({}));
    await createHttpApi({ baseUrl: "http://go:8080", getCookie: async () => "kerdos_session=abc", fetch: fetchImpl }).getMe();
    expect(calls[0].headers.get("cookie")).toBe("kerdos_session=abc");
  });

  it("turns the backend's error body into an ApiError", async () => {
    const { fetchImpl } = fakeFetch(() => json({ error: { code: "already_decided", message: "Already accepted." } }, 409));
    const err = await api(fetchImpl).acceptOffer("o1").catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ status: 409, code: "already_decided", message: "Already accepted." });
  });

  it("turns a non-JSON server error into an unexpected ApiError", async () => {
    const { fetchImpl } = fakeFetch(() => new Response("<html>oops</html>", { status: 502 }));
    expect(await api(fetchImpl).getMe().catch((e) => e)).toMatchObject({ status: 502, code: "unexpected" });
  });

  it("reports an unreachable backend as status 0", async () => {
    const fetchImpl = (async () => {
      throw new TypeError("fetch failed");
    }) as typeof fetch;
    expect(await api(fetchImpl).getMe().catch((e) => e)).toMatchObject({ status: 0, code: "unreachable" });
  });
});
