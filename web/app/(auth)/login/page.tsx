import { Suspense } from "react";
import type { Metadata } from "next";
import Link from "next/link";
import { redirect } from "next/navigation";
import { continueWithGoogle, signIn } from "@/app/(auth)/actions";
import { AuthCard, AuthCardFallback, Field, OrDivider } from "@/components/AuthCard";
import { apiErrorView } from "@/components/ErrorPanel";
import { FlashBanner } from "@/components/FlashBanner";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { getApi } from "@/lib/api";
import { safePath } from "@/lib/flash";

export const metadata: Metadata = { title: "Sign in · Kerdos" };

type Params = { next?: string; email?: string };

async function LoginContent({ searchParams }: { searchParams: Promise<Params> }) {
  const params = await searchParams;
  const next = safePath(params.next);
  let status;
  try {
    status = await getApi().getAuthStatus();
  } catch (error) {
    return apiErrorView(error);
  }
  if (status.mode === "dev") {
    return (
      <AuthCard title="Sign in" description="Invoices, reminders and a cash forecast for your business.">
        <div className="grid gap-3">
          <Button asChild variant="primary">
            <Link href="/">Continue to the demo</Link>
          </Button>
          <p className="text-sm text-muted">
            Sign-in is off on this server (<code>KERDOS_AUTH=dev</code>), so everyone acts as the owner. Set <code>KERDOS_AUTH=accounts</code> on
            the backend to turn accounts on.
          </p>
        </div>
      </AuthCard>
    );
  }
  if (status.user) redirect(next);

  return (
    <AuthCard title="Sign in" description="Invoices, reminders and a cash forecast for your business.">
      <FlashBanner className="" />
      {status.google ? (
        <>
          <form action={continueWithGoogle} className="grid">
            <input type="hidden" name="next" value={next} />
            <Button type="submit">Continue with Google</Button>
          </form>
          <OrDivider>or with your email</OrDivider>
        </>
      ) : null}
      <form action={signIn} className="grid gap-4">
        <input type="hidden" name="next" value={next} />
        <Field label="Email">
          <Input name="email" type="email" autoComplete="email" required defaultValue={params.email ?? ""} />
        </Field>
        <Field label="Password">
          <Input name="password" type="password" autoComplete="current-password" required />
        </Field>
        <Button type="submit" variant="primary">Sign in</Button>
      </form>
      {status.needs_setup ? (
        <p className="text-sm">
          Nobody has an account yet.{" "}
          <Link href="/signup" className="text-accent underline-offset-4 hover:underline">
            Create the owner account
          </Link>
        </p>
      ) : (
        <p className="text-sm text-muted">New here? Ask the owner of the business to invite you. The invitation email has a link to create your account.</p>
      )}
    </AuthCard>
  );
}

export default function LoginPage(props: { searchParams: Promise<Params> }) {
  return (
    <Suspense fallback={<AuthCardFallback />}>
      <LoginContent {...props} />
    </Suspense>
  );
}
