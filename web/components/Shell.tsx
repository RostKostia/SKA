import { Suspense, type ReactNode } from "react";
import { getApi } from "@/lib/api";
import { formatDate } from "@/lib/format";
import { apiErrorView } from "./ErrorPanel";
import { FlashBanner } from "./FlashBanner";
import { NavLinks } from "./NavLinks";

export function ShellFallback() {
  return <div className="shell" aria-busy="true" />;
}

export async function Shell({ children }: { children: ReactNode }) {
  let data;
  try {
    const api = getApi();
    data = await Promise.all([api.getMe(), api.getDashboard()]);
  } catch (error) {
    return apiErrorView(error);
  }
  const [user, dashboard] = data;

  return (
    <div className="shell">
      <nav className="shell__nav" aria-label="Main">
        <div className="shell__brand">
          <span aria-hidden="true">κ</span>
          <span>Kerdos</span>
        </div>
        <NavLinks />
      </nav>
      <div>
        <header className="shell__bar">
          <strong>{dashboard.company}</strong>
          <div className="shell__tools">
            <span className="muted">{formatDate(dashboard.today)}</span>
            <span>{user.name}</span>
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
