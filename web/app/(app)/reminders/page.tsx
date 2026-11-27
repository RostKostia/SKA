import { Suspense } from "react";
import { PageFallback } from "@/components/PageFallback";
import type { Metadata } from "next";
import { apiErrorView } from "@/components/ErrorPanel";
import { getApi } from "@/lib/api";
import { formatDate } from "@/lib/format";

export const metadata: Metadata = { title: "Reminders · Kerdos" };

const days = (n: number) => `${n} ${Math.abs(n) === 1 ? "day" : "days"}`;

async function RemindersPageContent() {
  const api = getApi();
  let data;
  try {
    data = await Promise.all([api.getReminderSettings(), api.listMessages()]);
  } catch (error) {
    return apiErrorView(error);
  }
  const [policy, messages] = data;

  return (
    <main className="shell__main">
      <h1>Reminders</h1>
      <p className="muted">
        Kerdos reminds clients by email on its own. To remind someone now, use Remind now on the Invoices or Debtors page.
      </p>

      <section className="slab stack" aria-labelledby="rules-title">
        <h2 id="rules-title">How reminders go out</h2>
        <ul>
          <li>{policy.enabled ? "Automatic reminders are on." : "Automatic reminders are off."}</li>
          {policy.before_due_days.length ? (
            <li>A friendly nudge {policy.before_due_days.map(days).join(" and ")} before the due date.</li>
          ) : null}
          <li>The first reminder {days(policy.first_after_due_days)} after the due date, then every {days(policy.every_days)}.</li>
          <li>The tone gets firmer each time: {policy.tone_steps.join(", then ")}.</li>
          <li>{policy.max_reminders ? `It stops after ${policy.max_reminders} reminders.` : "It keeps going until the invoice is paid or financed."}</li>
        </ul>
      </section>

      {policy.templates.length ? (
        <section className="stack" aria-labelledby="templates-title">
          <h2 id="templates-title">Email templates</h2>
          <p className="muted">They use {"{client_name}"}, {"{invoice_number}"}, {"{amount}"} and {"{due_date}"}.</p>
          <div className="cards">
            {policy.templates.map((t) => (
              <article key={t.id} className="slab stack">
                <h3>{t.tone ? `${t.kind}, ${t.tone}` : t.kind}</h3>
                <p><strong>{t.subject}</strong></p>
                <p style={{ whiteSpace: "pre-line" }}>{t.body}</p>
              </article>
            ))}
          </div>
        </section>
      ) : null}

      <section aria-labelledby="log-title" className="stack">
        <h2 id="log-title">Emails sent</h2>
        {messages.length === 0 ? (
          <p className="muted">Nothing sent yet.</p>
        ) : (
          <div className="table-wrap">
            <table className="table">
              <caption className="sr-only">Emails sent by Kerdos</caption>
              <thead>
                <tr>
                  <th scope="col">Date</th>
                  <th scope="col">To</th>
                  <th scope="col">Subject</th>
                  <th scope="col">Type</th>
                  <th scope="col">How</th>
                  <th scope="col">Status</th>
                </tr>
              </thead>
              <tbody>
                {messages.map((m) => (
                  <tr key={m.id}>
                    <td>{formatDate(m.business_date)}</td>
                    <td>{m.recipient}</td>
                    <th scope="row">{m.subject}</th>
                    <td>{m.kind}{m.tone ? `, ${m.tone}` : ""}</td>
                    <td>{m.mode === "automated" ? "Automatic" : "By you"}</td>
                    <td>{m.status}{m.error ? `: ${m.error}` : ""}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </main>
  );
}

export default function RemindersPage() {
  return (
    <Suspense fallback={<PageFallback />}>
      <RemindersPageContent />
    </Suspense>
  );
}
