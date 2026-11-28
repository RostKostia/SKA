import { Suspense } from "react";
import type { Metadata } from "next";
import Link from "next/link";
import { redirect } from "next/navigation";
import { continueWithGoogle, signIn } from "@/app/(auth)/actions";
import { apiErrorView } from "@/components/ErrorPanel";
import { FlashBanner } from "@/components/FlashBanner";
import { Button } from "@/components/ui/Button";
import { getApi } from "@/lib/api";
import { safePath } from "@/lib/flash";

export const metadata: Metadata = { title: "Sign in · Kerdos" };

type Params = { next?: string; email?: string };

async function SignInContent({ searchParams }: { searchParams: Promise<Params> }) {
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
      <main className="slab auth__card stack">
        <h1>Sign-in is off</h1>
        <p>This server runs with <code>KERDOS_AUTH=dev</code>, so everyone acts as the owner. Set <code>KERDOS_AUTH=accounts</code> on the backend to turn accounts on.</p>
        <Link href="/" className="key key--primary">Open Kerdos</Link>
      </main>
    );
  }
  if (status.user) redirect(next);

  return (
    <main className="slab auth__card stack">
      <h1>Sign in</h1>
      <FlashBanner />
      {status.google ? (
        <>
          <form action={continueWithGoogle}>
            <input type="hidden" name="next" value={next} />
            <Button type="submit" className="auth__wide">Continue with Google</Button>
          </form>
          <p className="auth__or muted">or with your email</p>
        </>
      ) : null}
      <form action={signIn} className="stack">
        <input type="hidden" name="next" value={next} />
        <label className="stack auth__label">
          Email
          <input className="field" name="email" type="email" autoComplete="email" required defaultValue={params.email ?? ""} />
        </label>
        <label className="stack auth__label">
          Password
          <input className="field" name="password" type="password" autoComplete="current-password" required />
        </label>
        <Button type="submit" variant="primary" className="auth__wide">Sign in</Button>
      </form>
      {status.needs_setup ? (
        <p>
          Nobody has an account yet. <Link href="/signup">Create the owner account</Link>
        </p>
      ) : (
        <p className="muted">New here? Ask the owner of the business to invite you. The invitation email has a link to create your account.</p>
      )}
    </main>
  );
}

export default function SignInPage(props: { searchParams: Promise<Params> }) {
  return (
    <Suspense fallback={<main className="slab auth__card" aria-busy="true" />}>
      <SignInContent {...props} />
    </Suspense>
  );
}
