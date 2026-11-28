import { describe, expect, it } from "vitest";
import type { Dashboard, Debtor, ForecastEvent } from "@/lib/api/types";
import { overviewKpis } from "./overview";

const debtor = (maxDaysOverdue: number): Debtor => ({
  counterparty_id: "c",
  name: "Client",
  open_invoices: 1,
  outstanding_cents: 10_000,
  oldest_age_days: 10,
  max_days_overdue: maxDaysOverdue,
  reminders_sent: 0,
  last_tone: null,
  oldest_invoice_id: "i",
});

const event = (day: number, kind: ForecastEvent["kind"], amount_cents: number): ForecastEvent => ({
  day,
  date: `2026-12-${String(day).padStart(2, "0")}`,
  amount_cents,
  kind,
  label: `${kind} ${day}`,
  invoice_id: null,
  cashflow_id: null,
});

function dashboard(debtors: Debtor[], events: ForecastEvent[]): Dashboard {
  return {
    company: "Kerdos Ltd",
    today: "2026-11-28",
    clock_offset_days: 0,
    demo: true,
    balance_cents: 100_000,
    owed_cents: 25_000,
    forecast: {
      today: "2026-11-28",
      horizon_days: 30,
      start_balance_cents: 100_000,
      points: [],
      events,
      shortfall: null,
      headline: "",
      method: "",
    },
    debtors,
    actions: [],
  };
}

describe("overviewKpis", () => {
  it("counts only clients with an overdue invoice", () => {
    const kpis = overviewKpis(dashboard([debtor(4), debtor(0)], []));
    expect(kpis.overdueClients).toBe(1);
  });

  it("passes the total owed through", () => {
    expect(overviewKpis(dashboard([], [])).owedCents).toBe(25_000);
  });

  it("picks the earliest incoming receivable as the next payment", () => {
    const kpis = overviewKpis(dashboard([], [event(2, "payable", -5_000), event(3, "receivable", 8_000), event(9, "receivable", 4_000)]));
    expect(kpis.nextPayment).toEqual({ date: "2026-12-03", label: "receivable 3", amount_cents: 8_000 });
  });

  it("has no next payment when no receivable is expected", () => {
    expect(overviewKpis(dashboard([], [event(2, "payable", -5_000)])).nextPayment).toBeNull();
  });
});
