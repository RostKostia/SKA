"use server";

import { redirect } from "next/navigation";
import { getApi } from "@/lib/api";
import { ApiError } from "@/lib/api/client";
import { safePath } from "@/lib/flash";
import { currentWebOrigin, startSession } from "@/lib/session";

const text = (data: FormData, name: string) => String(data.get(name) ?? "").trim();

// Back to a sign-in form with the backend's message, keeping what was typed (never the password).
function back(path: string, params: Record<string, string>): never {
  const url = new URL(path, "http://local");
  for (const [key, value] of Object.entries(params)) if (value) url.searchParams.set(key, value);
  redirect(url.pathname + url.search);
}

export async function signIn(data: FormData) {
  const next = safePath(data.get("next"));
  const email = text(data, "email");
  let token: string;
  try {
    ({ token } = await getApi().logIn(email, String(data.get("password") ?? "")));
  } catch (error) {
    if (!(error instanceof ApiError)) throw error;
    back("/signin", { error: error.message, email, next: next === "/" ? "" : next });
  }
  await startSession(token);
  redirect(next);
}

export async function signUp(data: FormData) {
  const invite = text(data, "invite");
  const name = text(data, "name");
  const email = text(data, "email");
  let token: string;
  try {
    ({ token } = await getApi().signUp({ name, email, password: String(data.get("password") ?? ""), invite: invite || undefined }));
  } catch (error) {
    if (!(error instanceof ApiError)) throw error;
    back("/signup", { error: error.message, invite, name, email });
  }
  await startSession(token);
  redirect("/");
}

// "Continue with Google": the backend gives the Google page to go to, and Google comes back
// to /auth/callback.
export async function continueWithGoogle(data: FormData) {
  const next = safePath(data.get("next"));
  const invite = text(data, "invite");
  const from = invite ? "/signup" : "/signin";
  let url: string;
  try {
    const returnTo = new URL("/auth/callback", await currentWebOrigin());
    returnTo.searchParams.set("next", next);
    url = await getApi().startGoogle(returnTo.toString(), invite || undefined);
  } catch (error) {
    if (!(error instanceof ApiError)) throw error;
    back(from, { error: error.message, invite });
  }
  redirect(url);
}
