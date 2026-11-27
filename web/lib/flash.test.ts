import { describe, expect, it } from "vitest";
import { flash, safePath } from "./flash";

describe("safePath", () => {
  it("keeps same-site paths with their query", () => {
    expect(safePath("/invoices?direction=payable&q=x")).toBe("/invoices?direction=payable&q=x");
    expect(safePath("/")).toBe("/");
  });

  it("falls back to the home page for anything that could leave the site", () => {
    for (const bad of ["//evil.example", "https://evil.example", "/\\evil.example", "javascript:alert(1)", "", null, undefined, 5]) {
      expect(safePath(bad)).toBe("/");
    }
  });
});

describe("flash", () => {
  it("adds a notice or an error to the query, encoded", () => {
    expect(flash("/invoices", { notice: "Synced 8 invoices & done" })).toBe(
      "/invoices?notice=Synced+8+invoices+%26+done",
    );
    expect(flash("/forecast", { error: "No." })).toBe("/forecast?error=No.");
  });

  it("keeps other query parameters and replaces an earlier message", () => {
    expect(flash("/invoices?direction=payable&error=old", { notice: "ok" })).toBe(
      "/invoices?direction=payable&notice=ok",
    );
  });

  it("clears messages when given none", () => {
    expect(flash("/invoices?notice=old", {})).toBe("/invoices");
  });
});
