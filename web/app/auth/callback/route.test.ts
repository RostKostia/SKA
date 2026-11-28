import { NextRequest } from "next/server";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api/client";

const backend = vi.hoisted(() => ({
  codes: [] as string[],
  impl: (async () => ({ user: { id: "u1" }, token: "session-token" })) as (code: string) => Promise<unknown>,
}));
vi.mock("@/lib/api", () => ({
  getApi: () => ({
    exchangeLoginCode: (code: string) => {
      backend.codes.push(code);
      return backend.impl(code);
    },
  }),
}));

import { GET } from "./route";

const call = (query: string) => GET(new NextRequest(`http://localhost:3000/auth/callback?${query}`, { headers: { host: "localhost:3000" } }));
const location = (res: Response) => new URL(res.headers.get("location")!);

beforeEach(() => {
  backend.codes.length = 0;
  backend.impl = async () => ({ user: { id: "u1" }, token: "session-token" });
});

describe("GET /auth/callback", () => {
  it("swaps the login code for a session cookie and goes on", async () => {
    const res = await call("login_code=abc&next=%2Finvoices");
    expect(backend.codes).toEqual(["abc"]);
    expect(location(res).pathname).toBe("/invoices");
    const cookie = res.headers.get("set-cookie")!;
    expect(cookie).toMatch(/^kerdos_session=session-token;/);
    expect(cookie).toMatch(/HttpOnly/i);
    expect(cookie).toMatch(/SameSite=lax/i);
  });

  it("never sends the browser off the site", async () => {
    expect(location(await call("login_code=abc&next=%2F%2Fevil.example")).toString()).toBe("http://localhost:3000/");
  });

  it("shows Google's error on the sign-in page, or on the account page when linking", async () => {
    let to = location(await call("auth_error=invite_required&auth_message=Ask+the+owner"));
    expect([to.pathname, to.searchParams.get("error")]).toEqual(["/signin", "Ask the owner"]);
    to = location(await call("auth_error=google_in_use&auth_message=In+use&next=%2Faccount&link=1"));
    expect([to.pathname, to.searchParams.get("error")]).toEqual(["/account", "In use"]);
    expect(backend.codes).toEqual([]);
  });

  it("confirms a linked Google account", async () => {
    const to = location(await call("google=linked&next=%2Faccount&link=1"));
    expect(to.pathname).toBe("/account");
    expect(to.searchParams.get("notice")).toMatch(/linked/);
  });

  it("sends an expired code back to sign in", async () => {
    backend.impl = async () => {
      throw new ApiError(400, "login_code_invalid", "This sign-in has expired. Sign in again.");
    };
    const res = await call("login_code=old");
    expect(location(res).searchParams.get("error")).toBe("This sign-in has expired. Sign in again.");
    expect(res.headers.get("set-cookie")).toBeNull();
  });
});
