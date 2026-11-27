import { Suspense } from "react";
import { PageFallback } from "@/components/PageFallback";
import type { Metadata } from "next";
import { acceptOffer, sendChat } from "@/app/(app)/actions";
import { apiErrorView } from "@/components/ErrorPanel";
import { ScrollToEnd } from "@/components/ScrollToEnd";
import { Button } from "@/components/ui/Button";
import { Paper } from "@/components/ui/Paper";
import { StatusStamp } from "@/components/ui/StatusStamp";
import { getApi } from "@/lib/api";
import type { ChatMessage, Invoice } from "@/lib/api/types";
import { formatDate, formatMoney, formatMoneyCompact } from "@/lib/format";

export const metadata: Metadata = { title: "Chat · Kerdos" };

const EXAMPLES = ["Invoice Kostas €120", "Who owes me?", "Will I go negative?"];

function SendKey({ text, variant }: { text: string; variant?: "primary" }) {
  return (
    <form action={sendChat}>
      <input type="hidden" name="text" value={text} />
      <input type="hidden" name="returnTo" value="/chat" />
      <Button type="submit" variant={variant}>{text}</Button>
    </form>
  );
}

function InvoiceCard({ invoice }: { invoice: Invoice }) {
  return (
    <Paper stub={<StatusStamp status={invoice.display_status} compact />}>
      <h3>{invoice.number}</h3>
      <p>{invoice.counterparty_name}</p>
      <p className="slip__amount">{formatMoney(invoice.amount_cents, invoice.currency)}</p>
      <p>Due {formatDate(invoice.due_date)}</p>
    </Paper>
  );
}

// What the backend asks the UI to draw under a message.
function Card({ message }: { message: ChatMessage }) {
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
        <Button type="submit" variant="primary">Accept {formatMoneyCompact(card.amount_cents ?? 0, "EUR")}</Button>
      </form>
    );
  }
  if (card.type === "suggestions" && card.chips?.length) {
    return <div className="row">{card.chips.map((chip) => <SendKey key={chip} text={chip} />)}</div>;
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
    <main className="shell__main chat">
      <h1>Chat</h1>
      {messages.length === 0 ? (
        <p className="muted">Tell Kerdos what to do, or try one of these.</p>
      ) : (
        <ol className="chat__log" aria-label="Conversation">
          {messages.map((m) =>
            m.role === "system" ? (
              <li key={m.id} className="chat__system muted">{m.text}</li>
            ) : (
              <li key={m.id} className={`chat__msg chat__msg--${m.role}`}>
                <div className={m.role === "owner" ? "well" : "slab"}>
                  <p>{m.text}</p>
                  <Card message={m} />
                </div>
              </li>
            ),
          )}
        </ol>
      )}
      <ScrollToEnd count={messages.length} />
      <div className="row">
        {EXAMPLES.map((example) => <SendKey key={example} text={example} />)}
      </div>
      <form action={sendChat} className="chat__form">
        <input type="hidden" name="returnTo" value="/chat" />
        <label htmlFor="chat-text" className="sr-only">Message Kerdos</label>
        <input id="chat-text" name="text" className="field" placeholder="Invoice Kostas, 3 hours at €40" autoComplete="off" required />
        <Button variant="primary" type="submit">Send</Button>
      </form>
    </main>
  );
}

export default function ChatPage() {
  return (
    <Suspense fallback={<PageFallback />}>
      <ChatPageContent />
    </Suspense>
  );
}
