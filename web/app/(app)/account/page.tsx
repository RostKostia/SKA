import { Suspense } from "react";
import type { Metadata } from "next";
import { changePassword, linkGoogle, signOut, signOutEverywhere, unlinkGoogle, updateProfile } from "@/app/(app)/actions";
import { apiErrorView } from "@/components/ErrorPanel";
import { PageFallback } from "@/components/PageFallback";
import { Button } from "@/components/ui/Button";
import { getApi } from "@/lib/api";
import { formatDate } from "@/lib/format";

export const metadata: Metadata = { title: "Your account · Kerdos" };

async function AccountPageContent() {
  const api = getApi();
  let data;
  try {
    data = await Promise.all([api.getMe(), api.getAuthStatus()]);
  } catch (error) {
    return apiErrorView(error);
  }
  const [me, status] = data;
  const accounts = status.mode === "accounts";

  return (
    <main className="shell__main">
      <h1>Your account</h1>
      <p className="muted">
        {me.role === "admin" ? "You own this business." : "You're a member of this business."} Account created{" "}
        {formatDate(me.created_at.slice(0, 10))}.
      </p>
      {!accounts ? (
        <p className="banner">Sign-in is off on this server (KERDOS_AUTH=dev): everyone acts as the owner, so changes here are for testing.</p>
      ) : null}

      <section className="slab stack account__section" aria-labelledby="profile-title">
        <h2 id="profile-title">Name and email</h2>
        <form action={updateProfile} className="stack">
          <input type="hidden" name="returnTo" value="/account" />
          <label className="stack auth__label">
            Name
            <input className="field" name="name" autoComplete="name" required maxLength={100} defaultValue={me.name} />
          </label>
          <label className="stack auth__label">
            Email
            <input className="field" name="email" type="email" autoComplete="email" required defaultValue={me.email} />
          </label>
          {me.has_password ? (
            <label className="stack auth__label">
              Current password
              <input className="field" name="current_password" type="password" autoComplete="current-password" />
              <span className="muted auth__hint">Only needed to change your email.</span>
            </label>
          ) : null}
          <div>
            <Button type="submit" variant="primary">Save</Button>
          </div>
        </form>
      </section>

      <section className="slab stack account__section" aria-labelledby="password-title">
        <h2 id="password-title">{me.has_password ? "Password" : "Add a password"}</h2>
        {!me.has_password ? <p className="muted">You sign in with Google. Add a password to sign in with your email too.</p> : null}
        <form action={changePassword} className="stack">
          <input type="hidden" name="returnTo" value="/account" />
          {me.has_password ? (
            <label className="stack auth__label">
              Current password
              <input className="field" name="current_password" type="password" autoComplete="current-password" required />
            </label>
          ) : null}
          <label className="stack auth__label">
            New password
            <input className="field" name="new_password" type="password" autoComplete="new-password" required minLength={8} />
            <span className="muted auth__hint">At least 8 characters. Your other devices are signed out when it changes.</span>
          </label>
          <label className="stack auth__label">
            New password again
            <input className="field" name="confirm_password" type="password" autoComplete="new-password" required minLength={8} />
          </label>
          <div>
            <Button type="submit">{me.has_password ? "Change password" : "Add password"}</Button>
          </div>
        </form>
      </section>

      <section className="slab stack account__section" aria-labelledby="google-title">
        <h2 id="google-title">Google</h2>
        {me.google_linked ? (
          <>
            <p>Linked to {me.google_email || "your Google account"}. You can use Continue with Google to sign in.</p>
            {me.has_password ? (
              <form action={unlinkGoogle}>
                <input type="hidden" name="returnTo" value="/account" />
                <Button type="submit">Disconnect Google</Button>
              </form>
            ) : (
              <p className="muted">To disconnect Google, add a password first, so you can still sign in.</p>
            )}
          </>
        ) : status.google ? (
          <>
            <p>Link your Google account to sign in with Continue with Google.</p>
            <form action={linkGoogle}>
              <Button type="submit">Link Google account</Button>
            </form>
          </>
        ) : (
          <p className="muted">Google sign-in isn&apos;t set up on this server.</p>
        )}
      </section>

      {accounts ? (
        <section className="slab stack account__section" aria-labelledby="sessions-title">
          <h2 id="sessions-title">Signing out</h2>
          <div className="row">
            <form action={signOut}>
              <Button type="submit">Sign out</Button>
            </form>
            <form action={signOutEverywhere}>
              <Button type="submit">Sign out on every device</Button>
            </form>
          </div>
        </section>
      ) : null}
    </main>
  );
}

export default function AccountPage() {
  return (
    <Suspense fallback={<PageFallback />}>
      <AccountPageContent />
    </Suspense>
  );
}
