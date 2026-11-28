import { Suspense, type ReactNode } from "react";
import Link from "next/link";
import { redirect } from "next/navigation";
import { signOut } from "@/app/(app)/actions";
import { getApi } from "@/lib/api";
import { formatDate } from "@/lib/format";
import { apiErrorView } from "./ErrorPanel";
import { FlashBanner } from "./FlashBanner";
import { NavLinks } from "./NavLinks";
import { Button } from "./ui/Button";

export function ShellFallback() {
  return <div className="shell" aria-busy="true" />;
}

export async function Shell({ children }: { children: ReactNode }) {
  let data;
  try {
    const api = getApi();
    data = await Promise.all([api.getAuthStatus(), api.getDashboard()]);
  } catch (error) {
    return apiErrorView(error);
  }
  const [status, dashboard] = data;
  const user = status.user;
  if (!user) redirect("/signin");

  return (
    <div className="shell">
      <nav className="shell__nav" aria-label="Main">
        <div className="shell__brand">
          <span aria-hidden="true">κ</span>
          <span>Kerdos</span>
        </div>
        <NavLinks owner={user.role === "admin"} />
      </nav>
      <div>
        <header className="shell__bar">
          <strong>{dashboard.company}</strong>
          <div className="shell__tools">
            <span className="muted">{formatDate(dashboard.today)}</span>
            <Link href="/account" className="shell__user">{user.name}</Link>
            {status.mode === "accounts" ? (
              <form action={signOut}>
                <Button type="submit" className="key--small">Sign out</Button>
              </form>
            ) : null}
          </div>
        </header>
        <Suspense fallback={null}>
          <FlashBanner />
        </Suspense>
        {children}
      </div>
    </div>
  );
}
