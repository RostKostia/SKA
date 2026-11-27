import { Suspense } from "react";
import { PageFallback } from "@/components/PageFallback";
import type { Metadata } from "next";
import Link from "next/link";
import { cancelInvoice, remindInvoice } from "@/app/(app)/actions";
import { apiErrorView } from "@/components/ErrorPanel";
import { Button } from "@/components/ui/Button";
import { StatusStamp } from "@/components/ui/StatusStamp";
import { getApi } from "@/lib/api";
import type { Direction, DisplayStatus, Invoice } from "@/lib/api/types";
import { formatAmount, formatDate } from "@/lib/format";
import { fold } from "@/lib/search";

export const metadata: Metadata = { title: "Invoices · Kerdos" };

type Params = Record<string, string | string[] | undefined>;

const STATUSES: DisplayStatus[] = ["unpaid", "overdue", "partially_paid", "paid", "financed", "draft", "cancelled"];
const SORTS: Record<string, { label: string; key: (i: Invoice) => string | number }> = {
  due_date: { label: "Due date", key: (i) => i.due_date },
  issue_date: { label: "Issue date", key: (i) => i.issue_date },
  amount: { label: "Amount", key: (i) => i.amount_cents },
  number: { label: "Number", key: (i) => i.number },
  counterparty: { label: "Counterparty", key: (i) => i.counterparty_name },
};
const TABS: { direction: Direction; label: string }[] = [
  { direction: "receivable", label: "From the company" },
  { direction: "payable", label: "To the company" },
];
const OPEN: DisplayStatus[] = ["unpaid", "overdue", "partially_paid"];

const one = (value: string | string[] | undefined) => (Array.isArray(value) ? value[0] : value) ?? "";

async function InvoicesPageContent({ searchParams }: { searchParams: Promise<Params> }) {
  const params = await searchParams;
  const direction: Direction = one(params.direction) === "payable" ? "payable" : "receivable";
  const status = STATUSES.find((s) => s === one(params.status));
  const sort = SORTS[one(params.sort)] ? one(params.sort) : "due_date";
  const desc = one(params.order) === "desc";
  const needle = fold(one(params.q).trim());

  let invoices;
  try {
    invoices = await getApi().listInvoices({ direction, status });
  } catch (error) {
    return apiErrorView(error);
  }
  const key = SORTS[sort].key;
  const shown = invoices
    .filter((i) => !needle || fold(`${i.counterparty_name} ${i.number} ${i.description}`).includes(needle))
    .sort((a, b) => {
      const [x, y] = [key(a), key(b)];
      return (x < y ? -1 : x > y ? 1 : 0) * (desc ? -1 : 1);
    });

  const keep = new URLSearchParams();
  for (const name of ["q", "status", "sort", "order"]) if (one(params[name])) keep.set(name, one(params[name]));
  const here = (dir: Direction) => `/invoices?${new URLSearchParams({ direction: dir, ...Object.fromEntries(keep) })}`;
  const returnTo = here(direction);

  return (
    <main className="shell__main">
      <h1>Invoices</h1>

      <nav aria-label="Direction" className="tabs">
        {TABS.map((tab) => (
          <Link key={tab.direction} href={here(tab.direction)} aria-current={tab.direction === direction ? "page" : undefined}>
            {tab.label}
          </Link>
        ))}
      </nav>

      <form className="toolbar" role="search" action="/invoices">
        <input type="hidden" name="direction" value={direction} />
        <label>
          <span className="sr-only">Search invoices</span>
          <input className="field" name="q" defaultValue={one(params.q)} placeholder="Search by name, number or description" />
        </label>
        <label>
          Status
          <select className="field" name="status" defaultValue={status ?? ""}>
            <option value="">All</option>
            {STATUSES.map((s) => <option key={s} value={s}>{s.replace("_", " ")}</option>)}
          </select>
        </label>
        <label>
          Sort by
          <select className="field" name="sort" defaultValue={sort}>
            {Object.entries(SORTS).map(([value, s]) => <option key={value} value={value}>{s.label}</option>)}
          </select>
        </label>
        <label>
          Order
          <select className="field" name="order" defaultValue={desc ? "desc" : "asc"}>
            <option value="asc">Ascending</option>
            <option value="desc">Descending</option>
          </select>
        </label>
        <Button type="submit" variant="primary">Apply</Button>
      </form>

      <div className="row">
        <a className="key" href={`/export/invoices?format=csv&direction=${direction}`}>Export CSV</a>
      </div>

      {shown.length === 0 ? (
        <p className="muted">No invoices match. Clear the search or the status filter.</p>
      ) : (
        <div className="table-wrap">
          <table className="table">
            <caption className="sr-only">{direction === "receivable" ? "Invoices from the company" : "Invoices to the company"}</caption>
            <thead>
              <tr>
                <th scope="col">Number</th>
                <th scope="col">{direction === "receivable" ? "Client" : "Supplier"}</th>
                <th scope="col" className="num">Amount</th>
                <th scope="col">Currency</th>
                <th scope="col">Issued</th>
                <th scope="col">Due</th>
                <th scope="col">Status</th>
                <th scope="col"><span className="sr-only">Actions</span></th>
              </tr>
            </thead>
            <tbody>
              {shown.map((i) => (
                <tr key={i.id}>
                  <th scope="row">{i.number}</th>
                  <td>{i.counterparty_name}</td>
                  <td className="num">{formatAmount(i.amount_cents)}</td>
                  <td>{i.currency}</td>
                  <td>{formatDate(i.issue_date)}</td>
                  <td>{formatDate(i.due_date)}</td>
                  <td><StatusStamp status={i.display_status} compact /></td>
                  <td className="actions">
                    {i.direction === "receivable" && OPEN.includes(i.display_status) ? (
                      <form action={remindInvoice}>
                        <input type="hidden" name="id" value={i.id} />
                        <input type="hidden" name="returnTo" value={returnTo} />
                        <Button type="submit" className="key--small">Remind now</Button>
                      </form>
                    ) : null}
                    {i.status === "sent" && !i.is_financed && i.outstanding_cents > 0 ? (
                      <form action={cancelInvoice}>
                        <input type="hidden" name="id" value={i.id} />
                        <input type="hidden" name="returnTo" value={returnTo} />
                        <Button type="submit" className="key--small">Cancel</Button>
                      </form>
                    ) : null}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </main>
  );
}

export default function InvoicesPage(props: { searchParams: Promise<Params> }) {
  return (
    <Suspense fallback={<PageFallback />}>
      <InvoicesPageContent {...props} />
    </Suspense>
  );
}
