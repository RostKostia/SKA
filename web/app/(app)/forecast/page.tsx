import type { Metadata } from "next";
import { Suspense } from "react";
import { acceptOffer, requestOffer } from "@/app/(app)/actions";
import { apiErrorView } from "@/components/ErrorPanel";
import { ForecastChart } from "@/components/ForecastChart";
import { PageFallback } from "@/components/PageFallback";
import { PageHeader } from "@/components/PageHeader";
import { Button } from "@/components/ui/Button";
import { Card, CardContent, CardTitle } from "@/components/ui/Card";
import { Table, TBody, Td, Th, THead, Tr } from "@/components/ui/Table";
import { getApi } from "@/lib/api";
import type { Offer } from "@/lib/api/types";
import { formatDate, formatMoney, formatMoneyCompact } from "@/lib/format";

export const metadata: Metadata = { title: "Forecast · Kerdos" };

type Params = Record<string, string | string[] | undefined>;
const one = (value: string | string[] | undefined) => (Array.isArray(value) ? value[0] : value) ?? "";

function OfferPanel({ offer }: { offer: Offer }) {
  return (
    <Card aria-labelledby="offer-title" className="reveal border-accent/40" style={{ "--i": 2 } as React.CSSProperties}>
      <CardContent>
        <CardTitle id="offer-title" className="text-lg">Get {formatMoneyCompact(offer.amount_cents, "EUR")} now</CardTitle>
        <p>
          {offer.product === "invoice_financing" ? "An advance against your unpaid invoices" : "An overdraft"}, fee{" "}
          {formatMoneyCompact(offer.fee_cents, "EUR")}
          {offer.repayment_date ? `, repaid by ${formatDate(offer.repayment_date)}` : ""}. Invoices that back it stop getting reminders.
        </p>
        {offer.invoices.length ? (
          <ul className="grid list-disc gap-1 pl-5 text-sm">
            {offer.invoices.map((i) => (
              <li key={i.invoice_id}>
                {i.client}, {i.number}: {formatMoneyCompact(i.outstanding_cents, "EUR")}
              </li>
            ))}
          </ul>
        ) : null}
        {offer.note ? <p className="text-sm text-muted">{offer.note}</p> : null}
        {offer.status === "offered" ? (
          <form action={acceptOffer}>
            <input type="hidden" name="id" value={offer.id} />
            <input type="hidden" name="returnTo" value="/forecast" />
            <Button variant="primary" type="submit">Accept {formatMoneyCompact(offer.amount_cents, "EUR")}</Button>
          </form>
        ) : (
          <p>This offer is {offer.status}.</p>
        )}
      </CardContent>
    </Card>
  );
}

async function ForecastPageContent({ searchParams }: { searchParams: Promise<Params> }) {
  const offerId = one((await searchParams).offer);
  const api = getApi();
  let data;
  try {
    data = await Promise.all([api.getForecast(30), offerId ? api.getOffer(offerId) : Promise.resolve(null)]);
  } catch (error) {
    return apiErrorView(error);
  }
  const [forecast, offer] = data;

  return (
    <div className="grid grid-cols-[minmax(0,1fr)] gap-6">
      <PageHeader
        title={forecast.headline}
        description={
          <>
            Available now: {formatMoney(forecast.start_balance_cents, "EUR")}. {forecast.method}
          </>
        }
      />

      <Card aria-label="Balance forecast" className="reveal" style={{ "--i": 1 } as React.CSSProperties}>
        <ForecastChart forecast={forecast} />
      </Card>

      {offer ? (
        <OfferPanel offer={offer} />
      ) : forecast.shortfall ? (
        <Card className="reveal flex flex-wrap items-center justify-between gap-4 border-danger/40 bg-danger/10" style={{ "--i": 2 } as React.CSSProperties}>
          <p className="max-w-prose">
            The balance drops {formatMoneyCompact(forecast.shortfall.shortfall_cents, "EUR")} below zero. You can cover it now.
          </p>
          <form action={requestOffer}>
            <input type="hidden" name="returnTo" value="/forecast" />
            <Button variant="primary" type="submit">Get {formatMoneyCompact(forecast.shortfall.shortfall_cents, "EUR")} now</Button>
          </form>
        </Card>
      ) : null}

      {forecast.events.length ? (
        <section aria-labelledby="events-title" className="reveal grid gap-3" style={{ "--i": 3 } as React.CSSProperties}>
          <h2 id="events-title" className="text-base font-semibold">What the forecast counts</h2>
          <Table caption="Money expected in and out">
            <THead>
              <Tr>
                <Th>Date</Th>
                <Th>What</Th>
                <Th className="text-right">Amount</Th>
              </Tr>
            </THead>
            <TBody>
              {forecast.events.map((e, i) => (
                <Tr key={`${e.date}-${i}`}>
                  <Td className="whitespace-nowrap">{formatDate(e.date)}</Td>
                  <Th scope="row" className="text-left font-medium normal-case tracking-normal text-foreground">{e.label}</Th>
                  <Td className={e.amount_cents < 0 ? "whitespace-nowrap text-right text-danger" : "whitespace-nowrap text-right"}>
                    {formatMoney(e.amount_cents, "EUR")}
                  </Td>
                </Tr>
              ))}
            </TBody>
          </Table>
        </section>
      ) : null}
    </div>
  );
}

export default function ForecastPage(props: { searchParams: Promise<Params> }) {
  return (
    <Suspense fallback={<PageFallback />}>
      <ForecastPageContent {...props} />
    </Suspense>
  );
}
