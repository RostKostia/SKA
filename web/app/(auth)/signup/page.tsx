import { Suspense } from "react";
import type { Metadata } from "next";
import Link from "next/link";
import { redirect } from "next/navigation";
import { continueWithGoogle, signUp } from "@/app/(auth)/actions";
import { apiErrorView } from "@/components/ErrorPanel";
import { FlashBanner } from "@/components/FlashBanner";
import { Button } from "@/components/ui/Button";
import { getApi } from "@/lib/api";
import { ApiError } from "@/lib/api/client";
import type { InviteInfo } from "@/lib/api/types";
import { formatDate } from "@/lib/format";

export const metadata: Metadata = { title: "Create your account · Kerdos" };

type Params = { invite?: string; name?: string; email?: string };

async function SignUpContent({ searchParams }: { searchParams: Promise<Params> }) {
  const params = await searchParams;
  const api = getApi();
  let status;
  let invite: InviteInfo | null = null;
  let inviteProblem = "";
  try {
    status = await api.getAuthStatus();
    if (params.invite) {
      try {
        invite = await api.getInvite(params.invite);
      } catch (error) {
        if (!(error instanceof ApiError) || error.status !== 404) throw error;
        inviteProblem = error.message;
      }
    }
  } catch (error) {
    return apiErrorView(error);
  }
  if (status.mode === "dev" || status.user) redirect("/");

  if (inviteProblem || (!invite && !status.needs_setup)) {
    return (
      <main className="slab auth__card stack">
        <h1>Kerdos is invite-only</h1>
        <p>{inviteProblem || "Ask the owner of the business to invite you. The invitation email has a link to create your account."}</p>
        <Link href="/signin" className="key">Sign in instead</Link>
      </main>
    );
  }

  return (
    <main className="slab auth__card stack">
      {invite ? (
        <>
          <h1>Join {invite.company}</h1>
          <p className="muted">
            {invite.invited_by ? `${invite.invited_by} invited you` : "You're invited"} to Kerdos. The invitation is for {invite.email} and
            works until {formatDate(invite.expires_at.slice(0, 10))}.
          </p>
        </>
      ) : (
        <>
          <h1>Create the owner account</h1>
          <p className="muted">You&apos;re the first one here, so this account owns the business. You can invite your team afterwards.</p>
        </>
      )}
      <FlashBanner />
      {status.google ? (
        <>
          <form action={continueWithGoogle}>
            <input type="hidden" name="invite" value={params.invite ?? ""} />
            <input type="hidden" name="next" value="/" />
            <Button type="submit" className="auth__wide">Continue with Google</Button>
          </form>
          <p className="auth__or muted">or choose a password</p>
        </>
      ) : null}
      <form action={signUp} className="stack">
        <input type="hidden" name="invite" value={params.invite ?? ""} />
        <label className="stack auth__label">
          Your name
          <input className="field" name="name" autoComplete="name" required maxLength={100} defaultValue={params.name ?? invite?.name ?? ""} />
        </label>
        <label className="stack auth__label">
          Email
          {invite ? (
            <input className="field" name="email" type="email" readOnly value={invite.email} />
          ) : (
            <input className="field" name="email" type="email" autoComplete="email" required defaultValue={params.email ?? ""} />
          )}
        </label>
        <label className="stack auth__label">
          Password
          <input className="field" name="password" type="password" autoComplete="new-password" required minLength={8} />
          <span className="muted auth__hint">At least 8 characters.</span>
        </label>
        <Button type="submit" variant="primary" className="auth__wide">Create account</Button>
      </form>
      <p className="muted">
        Already have an account? <Link href="/signin">Sign in</Link>
      </p>
    </main>
  );
}

export default function SignUpPage(props: { searchParams: Promise<Params> }) {
  return (
    <Suspense fallback={<main className="slab auth__card" aria-busy="true" />}>
      <SignUpContent {...props} />
    </Suspense>
  );
}
