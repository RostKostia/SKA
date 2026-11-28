import { Suspense } from "react";
import type { Metadata } from "next";
import Link from "next/link";
import { redirect } from "next/navigation";
import { continueWithGoogle, signUp } from "@/app/(auth)/actions";
import { AuthCard, AuthCardFallback, Field, OrDivider } from "@/components/AuthCard";
import { apiErrorView } from "@/components/ErrorPanel";
import { FlashBanner } from "@/components/FlashBanner";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { getApi } from "@/lib/api";
import { ApiError } from "@/lib/api/client";
import type { InviteInfo } from "@/lib/api/types";
import { formatDate } from "@/lib/format";
import { SIGN_IN_HREF } from "@/lib/nav";

export const metadata: Metadata = { title: "Create your account · Kerdos" };

type Params = { invite?: string; name?: string; email?: string };

const link = "text-accent underline-offset-4 hover:underline";

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
      <AuthCard
        title="Kerdos is invite-only"
        description={inviteProblem || "Ask the owner of the business to invite you. The invitation email has a link to create your account."}
      >
        <Button asChild>
          <Link href={SIGN_IN_HREF}>Sign in instead</Link>
        </Button>
      </AuthCard>
    );
  }

  const title = invite ? `Join ${invite.company}` : "Create the owner account";
  const description = invite
    ? `${invite.invited_by ? `${invite.invited_by} invited you` : "You're invited"} to Kerdos. The invitation is for ${invite.email} and works until ${formatDate(invite.expires_at.slice(0, 10))}.`
    : "You're the first one here, so this account owns the business. You can invite your team afterwards.";

  return (
    <AuthCard title={title} description={description}>
      <FlashBanner className="" />
      {status.google ? (
        <>
          <form action={continueWithGoogle} className="grid">
            <input type="hidden" name="invite" value={params.invite ?? ""} />
            <input type="hidden" name="next" value="/" />
            <Button type="submit">Continue with Google</Button>
          </form>
          <OrDivider>or choose a password</OrDivider>
        </>
      ) : null}
      <form action={signUp} className="grid gap-4">
        <input type="hidden" name="invite" value={params.invite ?? ""} />
        <Field label="Your name">
          <Input name="name" autoComplete="name" required maxLength={100} defaultValue={params.name ?? invite?.name ?? ""} />
        </Field>
        <Field label="Email">
          {invite ? (
            <Input name="email" type="email" readOnly value={invite.email} className="text-muted" />
          ) : (
            <Input name="email" type="email" autoComplete="email" required defaultValue={params.email ?? ""} />
          )}
        </Field>
        <Field label="Password" hint="At least 8 characters.">
          <Input name="password" type="password" autoComplete="new-password" required minLength={8} />
        </Field>
        <Button type="submit" variant="primary">Create account</Button>
      </form>
      <p className="text-sm text-muted">
        Already have an account?{" "}
        <Link href={SIGN_IN_HREF} className={link}>
          Sign in
        </Link>
      </p>
    </AuthCard>
  );
}

export default function SignUpPage(props: { searchParams: Promise<Params> }) {
  return (
    <Suspense fallback={<AuthCardFallback />}>
      <SignUpContent {...props} />
    </Suspense>
  );
}
