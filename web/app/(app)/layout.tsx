import { Suspense, type ReactNode } from "react";
import { Shell, ShellFallback } from "@/components/Shell";

export default function AppLayout({ children }: { children: ReactNode }) {
  return (
    <Suspense fallback={<ShellFallback />}>
      <Shell>{children}</Shell>
    </Suspense>
  );
}
