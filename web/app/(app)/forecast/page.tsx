import type { Metadata } from "next";
import { acceptOffer, requestOffer } from "@/app/(app)/actions";
import { apiErrorView } from "@/components/ErrorPanel";
import { ForecastChart } from "@/components/ForecastChart";
import { Button } from "@/components/ui/Button";
import { getApi } from "@/lib/api";
import type { Offer } from "@/lib/api/types";
import { formatDate, formatMoney, formatMoneyCompact } from "@/lib/format";

export const metadata: Metadata = { title: "Forecast · Kerdos" };

type Params = Record<string, string | string[] | undefined>;
const one = (value: string | string[] | undefined) => (Array.isArray(value) ? value[0] : value) ?? "";

function OfferPanel({ offer }: { offer: Offer }) {
  return (
    <section className="slab offer" aria-labelledby="offer-title">
      <h2 id="offer-title">Get {formatMoneyCompact(offer.amount_cents, "EUR")} now</h2>
      <p>
        {offer.product === "invoice_financing" ? "An advance against your unpaid invoices" : "An overdraft"}, fee{" "}
        {formatMoneyCompact(offer.fee_cents, "EUR")}
        {offer.repayment_date ? `, repaid by ${formatDate(offer.repayment_date)}` : ""}. Invoices that back it stop getting reminders.
      </p>
      {offer.invoices.length ? (
        <ul>
          {offer.invoices.map((i) => (
            <li key={i.invoice_id}>
              {i.client}, {i.number}: {formatMoneyCompact(i.outstanding_cents, "EUR")}
            </li>
          ))}
        </ul>
      ) : null}
      {offer.note ? <p className="muted">{offer.note}</p> : null}
      {offer.status === "offered" ? (
        <form action={acceptOffer}>
          <input type="hidden" name="id" value={offer.id} />
          <input type="hidden" name="returnTo" value="/forecast" />
          <Button variant="primary" type="submit">Accept {formatMoneyCompact(offer.amount_cents, "EUR")}</Button>
        </form>
      ) : (
        <p>This offer is {offer.status}.</p>
      )}
    </section>
  );
}

export default async function ForecastPage({ searchParams }: { searchParams: Promise<Params> }) {
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
    <main className="shell__main">
      <h1>{forecast.headline}</h1>
      <p className="muted">
        Available now: {formatMoney(forecast.start_balance_cents, "EUR")}. {forecast.method}
      </p>
      <section className="slab" aria-label="Balance forecast">
        <ForecastChart forecast={forecast} />
      </section>

      {offer ? (
        <OfferPanel offer={offer} />
      ) : forecast.shortfall ? (
        <form action={requestOffer}>
          <input type="hidden" name="returnTo" value="/forecast" />
          <Button variant="primary" type="submit">Get {formatMoneyCompact(forecast.shortfall.shortfall_cents, "EUR")} now</Button>
        </form>
      ) : null}

      {forecast.events.length ? (
        <section aria-labelledby="events-title" className="stack">
          <h2 id="events-title">What the forecast counts</h2>
          <div className="table-wrap">
            <table className="table">
              <caption className="sr-only">Money expected in and out</caption>
              <thead>
                <tr>
                  <th scope="col">Date</th>
                  <th scope="col">What</th>
                  <th scope="col" className="num">Amount</th>
                </tr>
              </thead>
              <tbody>
                {forecast.events.map((e, i) => (
                  <tr key={`${e.date}-${i}`}>
                    <td>{formatDate(e.date)}</td>
                    <th scope="row">{e.label}</th>
                    <td className="num">{formatMoney(e.amount_cents, "EUR")}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      ) : null}
    </main>
  );
}
