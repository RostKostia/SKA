import { describe, expect, it } from "vitest";
import { apiBaseUrl } from "./config";

describe("apiBaseUrl", () => {
  it("defaults to the Go backend on this machine", () => {
    expect(apiBaseUrl({})).toBe("http://127.0.0.1:8080");
  });

  it("uses KERDOS_API_URL without a trailing slash", () => {
    expect(apiBaseUrl({ KERDOS_API_URL: "https://api.example.com/" })).toBe("https://api.example.com");
  });

  it("refuses anything that is not an http(s) URL", () => {
    for (const bad of ["ftp://x", "not a url", "javascript:alert(1)"]) {
      expect(() => apiBaseUrl({ KERDOS_API_URL: bad })).toThrow(/KERDOS_API_URL/);
    }
  });
});
