import { describe, expect, it } from "vitest";
import { sessionCookie, webOrigin } from "./session";

describe("webOrigin", () => {
  it("prefers KERDOS_WEB_URL", () => {
    expect(webOrigin({ KERDOS_WEB_URL: "https://kerdos.example/" }, new Headers({ host: "x" }))).toBe("https://kerdos.example");
  });
  it("uses the request's host, http on localhost", () => {
    expect(webOrigin({}, new Headers({ host: "localhost:3000" }))).toBe("http://localhost:3000");
    expect(webOrigin({}, new Headers({ host: "kerdos.example", "x-forwarded-proto": "https" }))).toBe("https://kerdos.example");
  });
});

describe("sessionCookie", () => {
  it("is httpOnly and Lax, and Secure only over https", () => {
    expect(sessionCookie("t", "http://localhost:3000")).toMatchObject({ name: "kerdos_session", httpOnly: true, sameSite: "lax", secure: false });
    expect(sessionCookie("t", "https://kerdos.example").secure).toBe(true);
  });
});
