import type { Metadata } from "next";
import { Suspense } from "react";
import { apiErrorView } from "@/components/ErrorPanel";
import { PageFallback } from "@/components/PageFallback";
import { PageHeader } from "@/components/PageHeader";
import { Badge } from "@/components/ui/Badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Table, TBody, Td, Th, THead, Tr } from "@/components/ui/Table";
import { getApi } from "@/lib/api";
import { formatDate } from "@/lib/format";
import type { Tone } from "@/lib/status";

export const metadata: Metadata = { title: "Reminders · Kerdos" };

const days = (n: number) => `${n} ${Math.abs(n) === 1 ? "day" : "days"}`;
const step = (i: number) => ({ "--i": i }) as React.CSSProperties;

function emailTone(status: string): Tone {
  if (status === "sent") return "success";
  if (status === "failed") return "danger";
  return "neutral";
}

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
    <div className="grid grid-cols-[minmax(0,1fr)] gap-6">
      <PageHeader
        title="Reminders"
        description="Kerdos reminds clients by email on its own. To remind someone now, use Remind now on the Invoices or Receivables page."
      />

      <Card aria-labelledby="rules-title" className="reveal" style={step(1)}>
        <CardHeader>
          <CardTitle id="rules-title">How reminders go out</CardTitle>
          <Badge tone={policy.enabled ? "success" : "neutral"}>{policy.enabled ? "On" : "Off"}</Badge>
        </CardHeader>
        <ul className="grid list-disc gap-2 pl-5">
          <li>{policy.enabled ? "Automatic reminders are on." : "Automatic reminders are off."}</li>
          {policy.before_due_days.length ? (
            <li>A friendly nudge {policy.before_due_days.map(days).join(" and ")} before the due date.</li>
          ) : null}
          <li>The first reminder {days(policy.first_after_due_days)} after the due date, then every {days(policy.every_days)}.</li>
          <li>The tone gets firmer each time: {policy.tone_steps.join(", then ")}.</li>
          <li>{policy.max_reminders ? `It stops after ${policy.max_reminders} reminders.` : "It keeps going until the invoice is paid or financed."}</li>
        </ul>
      </Card>

      {policy.templates.length ? (
        <section className="reveal grid gap-3" aria-labelledby="templates-title" style={step(2)}>
          <div className="grid gap-1">
            <h2 id="templates-title" className="text-base font-semibold">Email templates</h2>
            <p className="text-sm text-muted">
              They use {"{client_name}"}, {"{invoice_number}"}, {"{amount}"} and {"{due_date}"}.
            </p>
          </div>
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
            {policy.templates.map((t) => (
              <Card key={t.id} className="min-w-0">
                <CardContent>
                  <h3 className="text-sm font-semibold capitalize">{(t.tone ? `${t.kind}, ${t.tone}` : t.kind).replace(/_/g, " ")}</h3>
                  <p className="font-medium [overflow-wrap:anywhere]">{t.subject}</p>
                  <p className="whitespace-pre-line text-sm text-muted [overflow-wrap:anywhere]">{t.body}</p>
                </CardContent>
              </Card>
            ))}
          </div>
        </section>
      ) : null}

      <section aria-labelledby="log-title" className="reveal grid gap-3" style={step(3)}>
        <h2 id="log-title" className="text-base font-semibold">Emails sent</h2>
        {messages.length === 0 ? (
          <Card>
            <p className="text-muted">Nothing sent yet.</p>
          </Card>
        ) : (
          <Table caption="Emails sent by Kerdos">
            <THead>
              <Tr>
                <Th>Date</Th>
                <Th>To</Th>
                <Th>Subject</Th>
                <Th>Type</Th>
                <Th>How</Th>
                <Th>Status</Th>
              </Tr>
            </THead>
            <TBody>
              {messages.map((m) => (
                <Tr key={m.id}>
                  <Td className="whitespace-nowrap">{formatDate(m.business_date)}</Td>
                  <Td><span className="block max-w-48 truncate" title={m.recipient}>{m.recipient}</span></Td>
                  <Th scope="row" className="text-left font-medium normal-case tracking-normal text-foreground">
                    <span className="block max-w-64 truncate" title={m.subject}>{m.subject}</span>
                  </Th>
                  <Td className="whitespace-nowrap">{m.kind}{m.tone ? `, ${m.tone}` : ""}</Td>
                  <Td className="whitespace-nowrap">{m.mode === "automated" ? "Automatic" : "By you"}</Td>
                  <Td>
                    <Badge tone={emailTone(m.status)}>{m.status}</Badge>
                    {m.error ? <span className="ml-2 text-sm text-danger">{m.error}</span> : null}
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

export default function RemindersPage() {
  return (
    <Suspense fallback={<PageFallback />}>
      <RemindersPageContent />
    </Suspense>
  );
}
