"use server";

import { revalidatePath } from "next/cache";
import { redirect } from "next/navigation";
import { getApi } from "@/lib/api";
import { ApiError } from "@/lib/api/client";
import { flash, safePath } from "@/lib/flash";
import { formatMoneyCompact } from "@/lib/format";

const text = (data: FormData, name: string) => String(data.get(name) ?? "").trim();

type Outcome = string | { notice?: string; params: Record<string, string> } | void;

// Runs one backend call for a form, then goes back to the page the form came from with a notice,
// or with the backend's error message.
async function run(data: FormData, work: () => Promise<Outcome>): Promise<never> {
  const returnTo = safePath(data.get("returnTo"));
  let outcome: Outcome;
  try {
    outcome = await work();
  } catch (error) {
    if (!(error instanceof ApiError)) throw error;
    redirect(flash(returnTo, { error: error.message }));
  }
  revalidatePath("/", "layout");
  const notice = typeof outcome === "string" ? outcome : outcome?.notice;
  const url = new URL(flash(returnTo, { notice }), "http://local");
  if (outcome && typeof outcome === "object") {
    for (const [key, value] of Object.entries(outcome.params)) url.searchParams.set(key, value);
  }
  redirect(url.pathname + url.search);
}

export async function sendChat(data: FormData) {
  await run(data, async () => {
    await getApi().sendChat(text(data, "text"));
  });
}

export async function remindInvoice(data: FormData) {
  await run(data, async () => {
    const message = await getApi().remindInvoice(text(data, "id"));
    return `Reminder sent to ${message.recipient}${message.invoice_number ? ` for ${message.invoice_number}` : ""}.`;
  });
}

export async function cancelInvoice(data: FormData) {
  await run(data, async () => {
    const invoice = await getApi().cancelInvoice(text(data, "id"));
    return `Cancelled ${invoice.number}.`;
  });
}

// Shows the offer (with its fee) before the owner accepts it.
export async function requestOffer(data: FormData) {
  await run(data, async () => {
    const offer = await getApi().createOffer();
    return { params: { offer: offer.id } };
  });
}

export async function acceptOffer(data: FormData) {
  await run(data, async () => {
    const { offer } = await getApi().acceptOffer(text(data, "id"));
    return `${formatMoneyCompact(offer.amount_cents, "EUR")} is on its way to your account.`;
  });
}
