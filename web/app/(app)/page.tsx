import Link from "next/link";
import { apiErrorView } from "@/components/ErrorPanel";
import { Paper } from "@/components/ui/Paper";
import { StatusStamp } from "@/components/ui/StatusStamp";
import { getApi } from "@/lib/api";
import type { DisplayStatus } from "@/lib/api/types";
import { formatDate, formatMoney } from "@/lib/format";

const OPEN: DisplayStatus[] = ["unpaid", "overdue", "partially_paid"];

export default async function Overview() {
  const api = getApi();
  let data;
  try {
    data = await Promise.all([api.getDashboard(), api.listInvoices({ direction: "receivable" })]);
  } catch (error) {
    return apiErrorView(error);
  }
  const [dashboard, invoices] = data;
  const open = invoices.filter((i) => OPEN.includes(i.display_status)).sort((a, b) => a.due_date.localeCompare(b.due_date));
  const overdue = open.filter((i) => i.display_status === "overdue").length;

  return (
    <main className="shell__main">
      <h1>Who owes you</h1>
      <section className="slab" aria-label="Total owed" style={{ maxWidth: "30rem" }}>
        <p style={{ fontSize: "2.5rem", fontWeight: 800, lineHeight: 1.1 }}>{formatMoney(dashboard.owed_cents, "EUR")}</p>
        <p className="muted">
          {open.length === 0
            ? "No open invoices."
            : `${open.length} open ${open.length === 1 ? "invoice" : "invoices"}, ${overdue} overdue.`}
        </p>
      </section>
      {dashboard.forecast.shortfall ? (
        <p className="banner banner--error" style={{ margin: 0 }}>
          {dashboard.forecast.headline} <Link href="/forecast">See the forecast</Link>
        </p>
      ) : null}
      <ul style={{ listStyle: "none", margin: 0, padding: 0, display: "grid", gap: "1.25rem", maxWidth: "44rem" }}>
        {open.map((invoice, index) => (
          <li key={invoice.id}>
            <Paper tilt={index % 2 ? 0.25 : -0.25} stub={<StatusStamp status={invoice.display_status} />}>
              <h2>{invoice.counterparty_name}</h2>
              <p>{invoice.number}</p>
              <p className="slip__amount">{formatMoney(invoice.outstanding_cents, invoice.currency)}</p>
              <p>Due {formatDate(invoice.due_date)}</p>
            </Paper>
          </li>
        ))}
      </ul>
    </main>
  );
}
