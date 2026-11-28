import type { Dashboard } from "@/lib/api/types";

export type NextPayment = { date: string; label: string; amount_cents: number };

export function overviewKpis(d: Dashboard): { owedCents: number; overdueClients: number; nextPayment: NextPayment | null } {
  const next = d.forecast.events
    .filter((e) => e.kind === "receivable" && e.amount_cents > 0)
    .sort((a, b) => a.day - b.day)[0];
  return {
    owedCents: d.owed_cents,
    overdueClients: d.debtors.filter((debtor) => debtor.max_days_overdue > 0).length,
    nextPayment: next ? { date: next.date, label: next.label, amount_cents: next.amount_cents } : null,
  };
}
