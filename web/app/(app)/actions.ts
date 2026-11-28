"use server";

import { revalidatePath } from "next/cache";
import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { getApi } from "@/lib/api";
import { ApiError } from "@/lib/api/client";
import { flash, safePath } from "@/lib/flash";
import { formatMoneyCompact } from "@/lib/format";
import { currentWebOrigin, endSession, INVITE_LINK_COOKIE } from "@/lib/session";

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
    if (error.status === 401) redirect("/signin");
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

// ---------------------------------------------------------------- signing out

export async function signOut() {
  try {
    await getApi().logOut();
  } catch (error) {
    if (!(error instanceof ApiError)) throw error; // unreachable backend: sign out here anyway
  }
  await endSession();
  redirect("/signin");
}

export async function signOutEverywhere() {
  try {
    await getApi().logOutEverywhere();
  } catch (error) {
    if (!(error instanceof ApiError)) throw error;
    if (error.status !== 401) redirect(flash("/account", { error: error.message }));
  }
  await endSession();
  redirect(flash("/signin", { notice: "You're signed out on every device." }));
}

// ---------------------------------------------------------------- the account page

export async function updateProfile(data: FormData) {
  await run(data, async () => {
    await getApi().updateMe({
      name: text(data, "name"),
      email: text(data, "email"),
      current_password: String(data.get("current_password") ?? "") || undefined,
    });
    return "Saved.";
  });
}

export async function changePassword(data: FormData) {
  await run(data, async () => {
    const next = String(data.get("new_password") ?? "");
    if (next !== String(data.get("confirm_password") ?? "")) {
      throw new ApiError(400, "mismatch", "The two new passwords don't match.");
    }
    await getApi().changePassword(String(data.get("current_password") ?? ""), next);
    return "Password saved. Your other devices are signed out.";
  });
}

export async function linkGoogle() {
  let url: string;
  try {
    const returnTo = new URL("/auth/callback", await currentWebOrigin());
    returnTo.searchParams.set("next", "/account");
    returnTo.searchParams.set("link", "1");
    url = await getApi().linkGoogle(returnTo.toString());
  } catch (error) {
    if (!(error instanceof ApiError)) throw error;
    if (error.status === 401) redirect("/signin");
    redirect(flash("/account", { error: error.message }));
  }
  redirect(url);
}

export async function unlinkGoogle(data: FormData) {
  await run(data, async () => {
    await getApi().unlinkGoogle();
    return "Google is disconnected. Sign in with your email and password from now on.";
  });
}

// ---------------------------------------------------------------- the team

export async function inviteMember(data: FormData) {
  await run(data, async () => {
    const { invite, link } = await getApi().invite(text(data, "email"), text(data, "name") || undefined);
    (await cookies()).set(INVITE_LINK_COOKIE, JSON.stringify({ email: invite.email, link }), {
      httpOnly: true,
      sameSite: "lax",
      path: "/team",
      maxAge: 600,
    });
    if (invite.email_status === "sent") return `Invitation sent to ${invite.email}.`;
    if (invite.email_status === "failed") {
      return `The invitation for ${invite.email} is ready, but the email didn't go out (${invite.email_error}). Send them the link below yourself.`;
    }
    return `The invitation for ${invite.email} is ready. The email is on its way.`;
  });
}

export async function revokeInvite(data: FormData) {
  await run(data, async () => {
    await getApi().revokeInvite(text(data, "id"));
    return `The invitation for ${text(data, "email")} is cancelled. Its link no longer works.`;
  });
}

export async function removeMember(data: FormData) {
  await run(data, async () => {
    await getApi().removeMember(text(data, "id"));
    return `${text(data, "name")} can no longer sign in.`;
  });
}
