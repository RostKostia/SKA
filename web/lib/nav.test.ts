import { describe, expect, it } from "vitest";
import { ACCOUNT_HREF, NAV_ITEMS, TEAM_ITEM } from "./nav";

describe("navigation", () => {
  it("has unique hrefs", () => {
    const hrefs = NAV_ITEMS.map((item) => item.href);
    expect(new Set(hrefs).size).toBe(hrefs.length);
  });

  it("labels the debtors page Receivables", () => {
    expect(NAV_ITEMS.find((item) => item.href === "/debtors")?.label).toBe("Receivables");
  });

  it("keeps the account page out of the main nav", () => {
    expect(NAV_ITEMS.some((item) => item.href === ACCOUNT_HREF)).toBe(false);
  });

  it("keeps the owner-only team page out of the shared list", () => {
    expect(NAV_ITEMS.some((item) => item.href === TEAM_ITEM.href)).toBe(false);
  });
});
