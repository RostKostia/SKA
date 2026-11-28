import type { Metadata } from "next";
import { Suspense, type CSSProperties } from "react";
import { changePassword, linkGoogle, signOut, signOutEverywhere, unlinkGoogle, updateProfile } from "@/app/(app)/actions";
import { Field } from "@/components/AuthCard";
import { apiErrorView } from "@/components/ErrorPanel";
import { PageFallback } from "@/components/PageFallback";
import { PageHeader } from "@/components/PageHeader";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { getApi } from "@/lib/api";
import { initials, roleLabel } from "@/lib/account";
import { formatDate } from "@/lib/format";

export const metadata: Metadata = { title: "Account · Kerdos" };

const step = (i: number) => ({ "--i": i }) as CSSProperties;

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
  const signsInWith = [me.has_password && "password", me.google_linked && "Google"].filter(Boolean).join(" and ");

  return (
    <div className="grid max-w-xl grid-cols-[minmax(0,1fr)] gap-6">
      <PageHeader title="Account" />
      {!accounts ? (
        <p role="status" className="reveal rounded-md border border-warning/40 bg-warning/10 px-4 py-3 text-sm text-warning" style={step(1)}>
          Sign-in is off on this server (KERDOS_AUTH=dev): everyone acts as the owner, so changes here are only for testing.
        </p>
      ) : null}

      <Card className="reveal flex items-center gap-4" style={step(1)}>
        <span aria-hidden="true" className="grid size-14 shrink-0 place-items-center rounded-full bg-surface-raised text-lg font-semibold">
          {initials(me.name)}
        </span>
        <div className="grid min-w-0 gap-1">
          <p className="truncate text-lg font-semibold">{me.name}</p>
          <p className="truncate text-sm text-muted">{me.email}</p>
          <div className="flex flex-wrap items-center gap-2 text-xs text-muted">
            <Badge tone="info">{roleLabel(me.role)}</Badge>
            <span>Since {formatDate(me.created_at.slice(0, 10))}</span>
            {signsInWith ? <span>· Signs in with {signsInWith}</span> : null}
          </div>
        </div>
      </Card>

      <Card className="reveal" style={step(2)} aria-labelledby="profile-title">
        <CardHeader>
          <CardTitle id="profile-title">Name and email</CardTitle>
        </CardHeader>
        <CardContent>
          <form action={updateProfile} className="grid gap-4">
            <input type="hidden" name="returnTo" value="/account" />
            <Field label="Name">
              <Input name="name" autoComplete="name" required maxLength={100} defaultValue={me.name} />
            </Field>
            <Field label="Email">
              <Input name="email" type="email" autoComplete="email" required defaultValue={me.email} />
            </Field>
            {me.has_password ? (
              <Field label="Current password" hint="Only needed to change your email.">
                <Input name="current_password" type="password" autoComplete="current-password" />
              </Field>
            ) : null}
            <div>
              <Button type="submit" variant="primary">Save</Button>
            </div>
          </form>
        </CardContent>
      </Card>

      <Card className="reveal" style={step(3)} aria-labelledby="password-title">
        <CardHeader>
          <CardTitle id="password-title">{me.has_password ? "Password" : "Add a password"}</CardTitle>
        </CardHeader>
        <CardContent>
          {!me.has_password ? <p className="text-sm text-muted">You sign in with Google. Add a password to sign in with your email too.</p> : null}
          <form action={changePassword} className="grid gap-4">
            <input type="hidden" name="returnTo" value="/account" />
            {me.has_password ? (
              <Field label="Current password">
                <Input name="current_password" type="password" autoComplete="current-password" required />
              </Field>
            ) : null}
            <Field label="New password" hint="At least 8 characters. Your other devices are signed out when it changes.">
              <Input name="new_password" type="password" autoComplete="new-password" required minLength={8} />
            </Field>
            <Field label="New password again">
              <Input name="confirm_password" type="password" autoComplete="new-password" required minLength={8} />
            </Field>
            <div>
              <Button type="submit">{me.has_password ? "Change password" : "Add password"}</Button>
            </div>
          </form>
        </CardContent>
      </Card>

      <Card className="reveal" style={step(4)} aria-labelledby="google-title">
        <CardHeader>
          <CardTitle id="google-title">Google</CardTitle>
          {me.google_linked ? <Badge tone="success">Linked</Badge> : null}
        </CardHeader>
        <CardContent>
          {me.google_linked ? (
            <>
              <p className="text-sm">
                Linked to <span className="break-all font-medium">{me.google_email || "your Google account"}</span>. You can use Continue with
                Google to sign in.
              </p>
              {me.has_password ? (
                <form action={unlinkGoogle}>
                  <input type="hidden" name="returnTo" value="/account" />
                  <Button type="submit">Disconnect Google</Button>
                </form>
              ) : (
                <p className="text-sm text-muted">To disconnect Google, add a password first, so you can still sign in.</p>
              )}
            </>
          ) : status.google ? (
            <>
              <p className="text-sm text-muted">Link your Google account to sign in with Continue with Google.</p>
              <form action={linkGoogle}>
                <Button type="submit">Link Google account</Button>
              </form>
            </>
          ) : (
            <p className="text-sm text-muted">Google sign-in isn&apos;t set up on this server.</p>
          )}
        </CardContent>
      </Card>

      {accounts ? (
        <Card className="reveal" style={step(5)} aria-labelledby="sessions-title">
          <CardHeader>
            <CardTitle id="sessions-title">Signing out</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex flex-wrap gap-3">
              <form action={signOut}>
                <Button type="submit">Sign out</Button>
              </form>
              <form action={signOutEverywhere}>
                <Button type="submit" variant="danger">Sign out on every device</Button>
              </form>
            </div>
          </CardContent>
        </Card>
      ) : null}
    </div>
  );
}

export default function AccountPage() {
  return (
    <Suspense fallback={<PageFallback />}>
      <AccountPageContent />
    </Suspense>
  );
}
