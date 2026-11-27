import { describe, expect, it } from "vitest";
import { invoicesToCsv } from "./csv";
import type { Invoice } from "./api/types";

const invoice = (over: Partial<Invoice>): Invoice => ({
  id: "inv_1",
  number: "KRD-2026-0004",
  direction: "receivable",
  counterparty_id: "cp_1",
  counterparty_name: "Elena's Bakery",
  description: "Menu and logo design",
  amount_cents: 45000,
  vat_cents: 0,
  paid_cents: 0,
  outstanding_cents: 45000,
  currency: "EUR",
  issue_date: "2026-11-17",
  due_date: "2026-12-01",
  status: "sent",
  display_status: "unpaid",
  days_overdue: 0,
  is_financed: false,
  source: "jinius",
  source_text: null,
  payment_link_url: null,
  sent_at: null,
  paid_at: null,
  ...over,
});

describe("invoicesToCsv", () => {
  it("starts with a BOM so Excel reads Greek names, then a header row", () => {
    const csv = invoicesToCsv([]);
    expect(csv.startsWith("﻿")).toBe(true);
    expect(csv.slice(1).trimEnd()).toBe("Number,Direction,Counterparty,Amount,Currency,Issue date,Due date,Status");
  });

  it("writes one row per invoice with amounts as decimals", () => {
    const rows = invoicesToCsv([invoice({})]).trimEnd().split("\r\n");
    expect(rows[1]).toBe("KRD-2026-0004,receivable,Elena's Bakery,450.00,EUR,2026-11-17,2026-12-01,unpaid");
  });

  it("quotes commas, quotes and newlines", () => {
    expect(invoicesToCsv([invoice({ counterparty_name: 'Smith, "Bob"\nLtd' })])).toContain('"Smith, ""Bob""\nLtd"');
  });

  it("defuses spreadsheet formulas in text cells", () => {
    expect(invoicesToCsv([invoice({ counterparty_name: "=SUM(A1)" })])).toContain("'=SUM(A1)");
    expect(invoicesToCsv([invoice({ number: "+1" })]).split("\r\n")[1].startsWith("'+1,")).toBe(true);
  });

  it("keeps Greek text intact", () => {
    expect(invoicesToCsv([invoice({ counterparty_name: "Μαρία Σάββα" })])).toContain("Μαρία Σάββα");
  });
});
