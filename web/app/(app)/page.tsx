import Link from "next/link";
import { Suspense, type CSSProperties } from "react";
import { apiErrorView } from "@/components/ErrorPanel";
import { PageFallback } from "@/components/PageFallback";
import { PageHeader } from "@/components/PageHeader";
import { StatusBadge } from "@/components/ui/Badge";
import { Card } from "@/components/ui/Card";
import { Table, TBody, Td, Th, THead, Tr } from "@/components/ui/Table";
import { getApi } from "@/lib/api";
import type { DisplayStatus } from "@/lib/api/types";
import { formatDate, formatMoney } from "@/lib/format";
import { overviewKpis } from "@/lib/overview";

const OPEN: DisplayStatus[] = ["unpaid", "overdue", "partially_paid"];

const step = (i: number) => ({ "--i": i }) as CSSProperties;

function Kpi({ label, value, hint, index }: { label: string; value: string; hint?: string; index: number }) {
  return (
    <Card className="reveal" style={step(index)}>
      <p className="text-sm text-muted">{label}</p>
      <p className="tabular mt-1 break-words text-3xl font-semibold tracking-tight">{value}</p>
      {hint ? <p className="mt-1 break-words text-sm text-muted">{hint}</p> : null}
    </Card>
  );
}

async function OverviewContent() {
  const api = getApi();
  let data;
  try {
    data = await Promise.all([api.getDashboard(), api.listInvoices({ direction: "receivable" })]);
  } catch (error) {
    return apiErrorView(error);
  }
  const [dashboard, invoices] = data;
  const open = invoices.filter((i) => OPEN.includes(i.display_status)).sort((a, b) => a.due_date.localeCompare(b.due_date));
  const kpis = overviewKpis(dashboard);

  return (
    <div className="grid grid-cols-[minmax(0,1fr)] gap-6">
      <PageHeader title="Overview" description={`${open.length} open ${open.length === 1 ? "invoice" : "invoices"}.`} />

      <div className="grid gap-4 sm:grid-cols-3">
        <Kpi index={1} label="Outstanding receivables" value={formatMoney(kpis.owedCents, "EUR")} />
        <Kpi index={2} label="Overdue clients" value={String(kpis.overdueClients)} />
        <Kpi
          index={3}
          label="Next expected payment"
          value={kpis.nextPayment ? formatMoney(kpis.nextPayment.amount_cents, "EUR") : "None expected"}
          hint={kpis.nextPayment ? `${kpis.nextPayment.label}, ${formatDate(kpis.nextPayment.date)}` : undefined}
        />
      </div>

      {dashboard.forecast.shortfall ? (
        <Card className="reveal border-danger/40 bg-danger/10" style={step(4)} role="status">
          <p>
            {dashboard.forecast.headline}{" "}
            <Link href="/forecast" className="font-medium text-accent underline underline-offset-4">
              See the forecast
            </Link>
          </p>
        </Card>
      ) : null}

      <section aria-labelledby="open-title" className="reveal grid gap-3" style={step(5)}>
        <h2 id="open-title" className="text-base font-semibold">Open invoices</h2>
        {open.length === 0 ? (
          <Card>
            <p className="text-muted">No open invoices.</p>
          </Card>
        ) : (
          <Table caption="Open receivable invoices">
            <THead>
              <Tr>
                <Th>Client</Th>
                <Th>Invoice</Th>
                <Th>Due</Th>
                <Th className="text-right">Outstanding</Th>
                <Th>Status</Th>
              </Tr>
            </THead>
            <TBody>
              {open.map((invoice) => (
                <Tr key={invoice.id}>
                  <Td className="font-medium">
                    <span className="block max-w-64 truncate" title={invoice.counterparty_name}>{invoice.counterparty_name}</span>
                  </Td>
                  <Td>{invoice.number}</Td>
                  <Td>{formatDate(invoice.due_date)}</Td>
                  <Td className="text-right">{formatMoney(invoice.outstanding_cents, invoice.currency)}</Td>
                  <Td>
                    <StatusBadge status={invoice.display_status} compact />
                  </Td>
                </Tr>
              ))}
            </TBody>
          </Table>
        )}
      </section>
    </div>
  );
}

export default function Overview() {
  return (
    <Suspense fallback={<PageFallback />}>
      <OverviewContent />
    </Suspense>
  );
}
