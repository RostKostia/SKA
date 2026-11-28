import Link from "next/link";
import { redirect } from "next/navigation";
import { Suspense, type ReactNode } from "react";
import { LogOut } from "lucide-react";
import { signOut } from "@/app/(app)/actions";
import { getApi } from "@/lib/api";
import { initials, roleLabel } from "@/lib/account";
import { formatDate } from "@/lib/format";
import { ACCOUNT_HREF, SIGN_IN_HREF } from "@/lib/nav";
import { apiErrorView } from "./ErrorPanel";
import { FlashBanner } from "./FlashBanner";
import { MobileNav } from "./MobileNav";
import { NavLinks } from "./NavLinks";
import { Button } from "./ui/Button";

function Brand() {
  return (
    <div className="flex items-center gap-2 text-lg font-semibold tracking-tight">
      <span aria-hidden="true" className="grid size-8 place-items-center rounded-md bg-accent text-accent-foreground">κ</span>
      Kerdos
    </div>
  );
}

export function ShellFallback() {
  return <div className="min-h-screen" aria-busy="true" />;
}

export async function Shell({ children }: { children: ReactNode }) {
  let data;
  try {
    const api = getApi();
    data = await Promise.all([api.getAuthStatus(), api.getDashboard()]);
  } catch (error) {
    return <main className="grid min-h-screen place-items-center p-6">{apiErrorView(error)}</main>;
  }
  const [status, dashboard] = data;
  const user = status.user;
  if (!user) redirect(SIGN_IN_HREF);
  const owner = user.role === "admin";
  // With KERDOS_AUTH=dev there is no sign-in, so there is nothing to sign out of.
  const canSignOut = status.mode === "accounts";

  return (
    <div className="min-h-screen md:grid md:grid-cols-[15rem_minmax(0,1fr)]">
      <aside className="sticky top-0 hidden h-screen flex-col gap-6 border-r border-border bg-surface p-4 md:flex">
        <Brand />
        <nav aria-label="Main" className="flex-1">
          <NavLinks owner={owner} />
        </nav>
        <div className="grid gap-1">
          <Link
            href={ACCOUNT_HREF}
            className="flex items-center gap-3 rounded-md px-2 py-2 transition-colors hover:bg-surface-raised"
          >
            <span aria-hidden="true" className="grid size-9 shrink-0 place-items-center rounded-full bg-surface-raised text-sm font-semibold">
              {initials(user.name)}
            </span>
            <span className="grid min-w-0 text-sm">
              <span className="truncate font-medium">{user.name}</span>
              <span className="truncate text-muted">{roleLabel(user.role)}</span>
            </span>
          </Link>
          {canSignOut ? (
            <form action={signOut}>
              <button
                type="submit"
                className="flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm font-medium text-muted transition-colors hover:bg-surface-raised hover:text-foreground"
              >
                <LogOut className="size-4" aria-hidden="true" />
                Sign out
              </button>
            </form>
          ) : null}
        </div>
      </aside>
      <div className="flex min-w-0 flex-col">
        <header className="sticky top-0 z-30 flex items-center gap-3 border-b border-border bg-background/85 px-4 py-3 backdrop-blur md:px-8">
          <MobileNav owner={owner} />
          <strong className="truncate">{dashboard.company}</strong>
          <span className="ml-auto shrink-0 text-sm text-muted">{formatDate(dashboard.today)}</span>
          <Link
            href={ACCOUNT_HREF}
            aria-label="Account"
            className="grid size-8 shrink-0 place-items-center rounded-full bg-surface-raised text-xs font-semibold md:hidden"
          >
            {initials(user.name)}
          </Link>
          {canSignOut ? (
            <form action={signOut} className="md:hidden">
              <Button type="submit" size="sm" aria-label="Sign out">
                <LogOut className="size-4" aria-hidden="true" />
              </Button>
            </form>
          ) : null}
        </header>
        <Suspense fallback={null}>
          <FlashBanner />
        </Suspense>
        <main className="min-w-0 flex-1 px-4 py-6 md:px-8">{children}</main>
      </div>
    </div>
  );
}
