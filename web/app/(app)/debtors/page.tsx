import { Suspense } from "react";
import { PageFallback } from "@/components/PageFallback";
import type { Metadata } from "next";
import Link from "next/link";
import { remindInvoice } from "@/app/(app)/actions";
import { apiErrorView } from "@/components/ErrorPanel";
import { Button } from "@/components/ui/Button";
import { getApi } from "@/lib/api";
import { formatMoney } from "@/lib/format";

export const metadata: Metadata = { title: "Debtors · Kerdos" };

async function DebtorsPageContent() {
  let debtors;
  try {
    debtors = await getApi().listDebtors();
  } catch (error) {
    return apiErrorView(error);
  }
  const total = debtors.reduce((sum, d) => sum + d.outstanding_cents, 0);

  return (
    <main className="shell__main">
      <h1>Who owes you</h1>
      {debtors.length === 0 ? (
        <p className="muted">Nobody owes you anything right now.</p>
      ) : (
        <>
          <p className="muted">
            {formatMoney(total, "EUR")} across {debtors.length} {debtors.length === 1 ? "client" : "clients"}. Financed invoices are not listed.
          </p>
          <div className="table-wrap">
            <table className="table">
              <caption className="sr-only">Clients with unpaid invoices</caption>
              <thead>
                <tr>
                  <th scope="col">Client</th>
                  <th scope="col" className="num">Owes</th>
                  <th scope="col" className="num">Open invoices</th>
                  <th scope="col" className="num">Days overdue</th>
                  <th scope="col">Reminders</th>
                  <th scope="col"><span className="sr-only">Actions</span></th>
                </tr>
              </thead>
              <tbody>
                {debtors.map((d) => (
                  <tr key={d.counterparty_id}>
                    <th scope="row">{d.name}</th>
                    <td className="num">{formatMoney(d.outstanding_cents, "EUR")}</td>
                    <td className="num">{d.open_invoices}</td>
                    <td className="num">{d.max_days_overdue > 0 ? d.max_days_overdue : "Not yet due"}</td>
                    <td>{d.reminders_sent === 0 ? "None yet" : `${d.reminders_sent}, last ${d.last_tone ?? ""}`}</td>
                    <td className="actions">
                      <form action={remindInvoice}>
                        <input type="hidden" name="id" value={d.oldest_invoice_id} />
                        <input type="hidden" name="returnTo" value="/debtors" />
                        <Button type="submit" className="key--small">Remind now</Button>
                      </form>
                      <Link href={`/invoices?q=${encodeURIComponent(d.name)}`}>Invoices</Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}
    </main>
  );
}

export default function DebtorsPage() {
  return (
    <Suspense fallback={<PageFallback />}>
      <DebtorsPageContent />
    </Suspense>
  );
}
