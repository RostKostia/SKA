import { describe, expect, it } from "vitest";
import { formatAmount, formatDate, formatMoney, formatMoneyCompact } from "./format";

// A timezone behind UTC would show "26 Nov" if a date-only string were parsed as local time.
process.env.TZ = "Pacific/Honolulu";

describe("formatMoney", () => {
  it("formats cents as euros with thousands separators", () => {
    expect(formatMoney(123450, "EUR")).toBe("€1,234.50");
  });
  it("puts the minus sign before the currency symbol", () => {
    expect(formatMoney(-80000, "EUR")).toBe("-€800.00");
  });
  it("formats zero", () => {
    expect(formatMoney(0, "EUR")).toBe("€0.00");
  });
});

describe("formatMoneyCompact", () => {
  it("drops the cents when the amount is whole euros", () => {
    expect(formatMoneyCompact(80000, "EUR")).toBe("€800");
    expect(formatMoneyCompact(-80000, "EUR")).toBe("-€800");
    expect(formatMoneyCompact(123400, "EUR")).toBe("€1,234");
  });
  it("keeps the cents otherwise", () => {
    expect(formatMoneyCompact(123450, "EUR")).toBe("€1,234.50");
    expect(formatMoneyCompact(12005, "EUR")).toBe("€120.05");
  });
});

describe("formatAmount", () => {
  it("formats a number with two decimals and no currency symbol, for a table column", () => {
    expect(formatAmount(45000)).toBe("450.00");
    expect(formatAmount(123456)).toBe("1,234.56");
    expect(formatAmount(-5)).toBe("-0.05");
  });
});

describe("formatDate", () => {
  it("keeps a date-only string on the same day in any timezone", () => {
    expect(formatDate("2026-11-27")).toBe("27 Nov 2026");
  });
  it("does not zero-pad the day", () => {
    expect(formatDate("2026-03-01")).toBe("1 Mar 2026");
  });
});
