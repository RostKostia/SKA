import type { Metadata } from "next";
import Link from "next/link";
import { Suspense } from "react";
import { cancelInvoice, remindInvoice } from "@/app/(app)/actions";
import { apiErrorView } from "@/components/ErrorPanel";
import { PageFallback } from "@/components/PageFallback";
import { PageHeader } from "@/components/PageHeader";
import { StatusBadge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Table, TBody, Td, Th, THead, Tr } from "@/components/ui/Table";
import { getApi } from "@/lib/api";
import type { Direction, DisplayStatus, Invoice } from "@/lib/api/types";
import { formatDate, formatMoney } from "@/lib/format";
import { fold } from "@/lib/search";
import { statusMeta } from "@/lib/status";
import { cn } from "@/lib/utils";

export const metadata: Metadata = { title: "Invoices · Kerdos" };

type Params = Record<string, string | string[] | undefined>;

const STATUSES: DisplayStatus[] = ["unpaid", "overdue", "partially_paid", "paid", "financed", "draft", "cancelled"];
const SORTS: Record<string, { label: string; key: (i: Invoice) => string | number }> = {
  due_date: { label: "Due date", key: (i) => i.due_date },
  issue_date: { label: "Issue date", key: (i) => i.issue_date },
  amount: { label: "Amount", key: (i) => i.amount_cents },
  number: { label: "Number", key: (i) => i.number },
  counterparty: { label: "Client or supplier", key: (i) => i.counterparty_name },
};
const TABS: { direction: Direction; label: string }[] = [
  { direction: "receivable", label: "Receivable" },
  { direction: "payable", label: "Payable" },
];
const OPEN: DisplayStatus[] = ["unpaid", "overdue", "partially_paid"];

const one = (value: string | string[] | undefined) => (Array.isArray(value) ? value[0] : value) ?? "";

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="grid min-w-36 gap-1 text-sm font-medium">
      {label}
      {children}
    </label>
  );
}

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
    <div className="grid grid-cols-[minmax(0,1fr)] gap-6">
      <PageHeader
        title="Invoices"
        actions={
          <Button asChild>
            <a href={`/export/invoices?format=csv&direction=${direction}`}>Export CSV</a>
          </Button>
        }
      />

      <nav aria-label="Direction" className="reveal inline-flex w-fit gap-1 rounded-lg border border-border bg-surface p-1" style={{ "--i": 1 } as React.CSSProperties}>
        {TABS.map((tab) => {
          const current = tab.direction === direction;
          return (
            <Link
              key={tab.direction}
              href={here(tab.direction)}
              aria-current={current ? "page" : undefined}
              className={cn(
                "rounded-md px-3 py-1.5 text-sm font-medium text-muted transition-colors hover:text-foreground",
                current && "bg-surface-raised text-foreground",
              )}
            >
              {tab.label}
            </Link>
          );
        })}
      </nav>

      <form className="reveal flex flex-wrap items-end gap-3" role="search" action="/invoices" style={{ "--i": 2 } as React.CSSProperties}>
        <input type="hidden" name="direction" value={direction} />
        <div className="min-w-60 flex-1">
          <label className="sr-only" htmlFor="invoice-search">Search invoices</label>
          <Input id="invoice-search" name="q" defaultValue={one(params.q)} placeholder="Search by name, number or description" />
        </div>
        <Field label="Status">
          <Select name="status" defaultValue={status ?? ""}>
            <option value="">All</option>
            {STATUSES.map((s) => (
              <option key={s} value={s}>{statusMeta(s).label}</option>
            ))}
          </Select>
        </Field>
        <Field label="Sort by">
          <Select name="sort" defaultValue={sort}>
            {Object.entries(SORTS).map(([value, s]) => (
              <option key={value} value={value}>{s.label}</option>
            ))}
          </Select>
        </Field>
        <Field label="Order">
          <Select name="order" defaultValue={desc ? "desc" : "asc"}>
            <option value="asc">Ascending</option>
            <option value="desc">Descending</option>
          </Select>
        </Field>
        <Button type="submit" variant="primary">Apply</Button>
      </form>

      {shown.length === 0 ? (
        <Card>
          <p className="text-muted">No invoices match. Clear the search or the status filter.</p>
        </Card>
      ) : (
        <div className="reveal" style={{ "--i": 3 } as React.CSSProperties}>
          <Table caption={direction === "receivable" ? "Receivable invoices" : "Payable invoices"}>
            <THead>
              <Tr>
                <Th>Number</Th>
                <Th>{direction === "receivable" ? "Client" : "Supplier"}</Th>
                <Th className="text-right">Amount</Th>
                <Th>Issued</Th>
                <Th>Due</Th>
                <Th>Status</Th>
                <Th><span className="sr-only">Actions</span></Th>
              </Tr>
            </THead>
            <TBody>
              {shown.map((i) => (
                <Tr key={i.id}>
                  <Th scope="row" className="whitespace-nowrap text-left font-medium normal-case tracking-normal text-foreground">{i.number}</Th>
                  <Td>
                    <span className="block max-w-56 truncate" title={i.counterparty_name}>{i.counterparty_name}</span>
                  </Td>
                  <Td className="whitespace-nowrap text-right">{formatMoney(i.amount_cents, i.currency)}</Td>
                  <Td className="whitespace-nowrap">{formatDate(i.issue_date)}</Td>
                  <Td className="whitespace-nowrap">{formatDate(i.due_date)}</Td>
                  <Td><StatusBadge status={i.display_status} compact /></Td>
                  <Td className="whitespace-nowrap text-right">
                    <div className="flex justify-end gap-2">
                      {i.direction === "receivable" && OPEN.includes(i.display_status) ? (
                        <form action={remindInvoice}>
                          <input type="hidden" name="id" value={i.id} />
                          <input type="hidden" name="returnTo" value={returnTo} />
                          <Button type="submit" size="sm">Remind now</Button>
                        </form>
                      ) : null}
                      {i.status === "sent" && !i.is_financed && i.outstanding_cents > 0 ? (
                        <form action={cancelInvoice}>
                          <input type="hidden" name="id" value={i.id} />
                          <input type="hidden" name="returnTo" value={returnTo} />
                          <Button type="submit" size="sm">Cancel</Button>
                        </form>
                      ) : null}
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

export default function InvoicesPage(props: { searchParams: Promise<Params> }) {
  return (
    <Suspense fallback={<PageFallback />}>
      <InvoicesPageContent {...props} />
    </Suspense>
  );
}
