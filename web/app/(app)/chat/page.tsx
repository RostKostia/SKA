import type { Metadata } from "next";
import { Suspense } from "react";
import { acceptOffer, sendChat } from "@/app/(app)/actions";
import { apiErrorView } from "@/components/ErrorPanel";
import { PageFallback } from "@/components/PageFallback";
import { PageHeader } from "@/components/PageHeader";
import { ScrollToEnd } from "@/components/ScrollToEnd";
import { StatusBadge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { getApi } from "@/lib/api";
import type { ChatMessage, Invoice } from "@/lib/api/types";
import { formatDate, formatMoney, formatMoneyCompact } from "@/lib/format";
import { cn } from "@/lib/utils";

export const metadata: Metadata = { title: "Chat · Kerdos" };

const EXAMPLES = ["Invoice Kostas €120", "Show outstanding receivables", "Will I go negative?"];

function SendKey({ text, variant }: { text: string; variant?: "primary" }) {
  return (
    <form action={sendChat}>
      <input type="hidden" name="text" value={text} />
      <input type="hidden" name="returnTo" value="/chat" />
      <Button type="submit" variant={variant} size="sm">{text}</Button>
    </form>
  );
}

function InvoiceCard({ invoice }: { invoice: Invoice }) {
  return (
    <div className="grid gap-1 rounded-md border border-border bg-background p-3">
      <div className="flex items-center justify-between gap-3">
        <h3 className="text-sm font-semibold">{invoice.number}</h3>
        <StatusBadge status={invoice.display_status} compact />
      </div>
      <p className="break-words text-sm">{invoice.counterparty_name}</p>
      <p className="tabular text-xl font-semibold">{formatMoney(invoice.amount_cents, invoice.currency)}</p>
      <p className="text-sm text-muted">Due {formatDate(invoice.due_date)}</p>
    </div>
  );
}

// What the backend asks the UI to draw under a message.
function MessageCard({ message }: { message: ChatMessage }) {
  const card = message.card;
  if (message.invoice) return <InvoiceCard invoice={message.invoice} />;
  if (!card) return null;
  if (card.type === "warning" && card.shortfall_cents) {
    return <SendKey text={`Get ${formatMoneyCompact(card.shortfall_cents, "EUR")} now`} variant="primary" />;
  }
  if (card.type === "offer" && card.offer_id) {
    return (
      <form action={acceptOffer}>
        <input type="hidden" name="id" value={card.offer_id} />
        <input type="hidden" name="returnTo" value="/chat" />
        <Button type="submit" variant="primary" size="sm">Accept {formatMoneyCompact(card.amount_cents ?? 0, "EUR")}</Button>
      </form>
    );
  }
  if (card.type === "suggestions" && card.chips?.length) {
    return <div className="flex flex-wrap gap-2">{card.chips.map((chip) => <SendKey key={chip} text={chip} />)}</div>;
  }
  return null;
}

async function ChatPageContent() {
  let messages;
  try {
    messages = await getApi().listChat();
  } catch (error) {
    return apiErrorView(error);
  }

  return (
    <div className="mx-auto grid max-w-3xl grid-cols-[minmax(0,1fr)] gap-6">
      <PageHeader title="Chat" description="Tell Kerdos what to do, or try one of the examples below." />
      {messages.length > 0 ? (
        <ol className="grid list-none gap-3 p-0" aria-label="Conversation">
          {messages.map((m) =>
            m.role === "system" ? (
              <li key={m.id} className="text-center text-sm text-muted">{m.text}</li>
            ) : (
              <li key={m.id} className={cn("flex", m.role === "owner" && "justify-end")}>
                <div
                  className={cn(
                    "grid max-w-[85%] gap-3 rounded-lg border px-4 py-3",
                    m.role === "owner" ? "border-accent/30 bg-accent/10" : "border-border bg-surface",
                  )}
                >
                  <p className="whitespace-pre-line [overflow-wrap:anywhere]">{m.text}</p>
                  <MessageCard message={m} />
                </div>
              </li>
            ),
          )}
        </ol>
      ) : null}
      <ScrollToEnd count={messages.length} />
      <div className="sticky bottom-0 -mx-4 grid gap-3 bg-background/90 px-4 pb-4 pt-3 backdrop-blur md:-mx-8 md:px-8">
        <div className="flex flex-wrap gap-2">
          {EXAMPLES.map((example) => <SendKey key={example} text={example} />)}
        </div>
        <form action={sendChat} className="flex gap-3">
          <input type="hidden" name="returnTo" value="/chat" />
          <label htmlFor="chat-text" className="sr-only">Message Kerdos</label>
          <Input id="chat-text" name="text" placeholder="Invoice Kostas, 3 hours at €40" autoComplete="off" required />
          <Button variant="primary" type="submit">Send</Button>
        </form>
      </div>
    </div>
  );
}

export default function ChatPage() {
  return (
    <Suspense fallback={<PageFallback />}>
      <ChatPageContent />
    </Suspense>
  );
}
