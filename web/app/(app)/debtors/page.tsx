import type { Metadata } from "next";
import Link from "next/link";
import { Suspense } from "react";
import { remindInvoice } from "@/app/(app)/actions";
import { apiErrorView } from "@/components/ErrorPanel";
import { PageFallback } from "@/components/PageFallback";
import { PageHeader } from "@/components/PageHeader";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Table, TBody, Td, Th, THead, Tr } from "@/components/ui/Table";
import { getApi } from "@/lib/api";
import { formatMoney } from "@/lib/format";

export const metadata: Metadata = { title: "Receivables · Kerdos" };

async function DebtorsPageContent() {
  let debtors;
  try {
    debtors = await getApi().listDebtors();
  } catch (error) {
    return apiErrorView(error);
  }
  const total = debtors.reduce((sum, d) => sum + d.outstanding_cents, 0);

  return (
    <div className="grid grid-cols-[minmax(0,1fr)] gap-6">
      <PageHeader
        title="Outstanding receivables"
        description={
          debtors.length === 0
            ? undefined
            : `${formatMoney(total, "EUR")} across ${debtors.length} ${debtors.length === 1 ? "client" : "clients"}. Financed invoices are not listed.`
        }
      />
      {debtors.length === 0 ? (
        <Card>
          <p className="text-muted">No outstanding receivables.</p>
        </Card>
      ) : (
        <div className="reveal" style={{ "--i": 1 } as React.CSSProperties}>
          <Table caption="Clients with unpaid invoices">
            <THead>
              <Tr>
                <Th>Client</Th>
                <Th className="text-right">Outstanding</Th>
                <Th className="text-right">Open invoices</Th>
                <Th>Overdue</Th>
                <Th>Reminders</Th>
                <Th><span className="sr-only">Actions</span></Th>
              </Tr>
            </THead>
            <TBody>
              {debtors.map((d) => (
                <Tr key={d.counterparty_id}>
                  <Th scope="row" className="text-left font-medium normal-case tracking-normal text-foreground">
                    <span className="block max-w-56 truncate" title={d.name}>{d.name}</span>
                  </Th>
                  <Td className="whitespace-nowrap text-right">{formatMoney(d.outstanding_cents, "EUR")}</Td>
                  <Td className="text-right">{d.open_invoices}</Td>
                  <Td>
                    {d.max_days_overdue > 0 ? (
                      <Badge tone="danger">{d.max_days_overdue} days</Badge>
                    ) : (
                      <span className="text-muted">Not yet due</span>
                    )}
                  </Td>
                  <Td className="whitespace-nowrap">
                    {d.reminders_sent === 0 ? "None yet" : `${d.reminders_sent}, last ${d.last_tone ?? ""}`}
                  </Td>
                  <Td>
                    <div className="flex items-center justify-end gap-3 whitespace-nowrap">
                      <form action={remindInvoice}>
                        <input type="hidden" name="id" value={d.oldest_invoice_id} />
                        <input type="hidden" name="returnTo" value="/debtors" />
                        <Button type="submit" size="sm">Remind now</Button>
                      </form>
                      <Link href={`/invoices?q=${encodeURIComponent(d.name)}`} className="text-sm text-accent underline-offset-4 hover:underline">
                        Invoices
                      </Link>
                    </div>
                  </Td>
                </Tr>
              ))}
            </TBody>
          </Table>
        </div>
      )}
    </div>
  );
}

export default function DebtorsPage() {
  return (
    <Suspense fallback={<PageFallback />}>
      <DebtorsPageContent />
    </Suspense>
  );
}
