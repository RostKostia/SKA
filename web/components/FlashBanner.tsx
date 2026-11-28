"use client";

import { useSearchParams } from "next/navigation";
import { cn } from "@/lib/utils";

// Shows the ?notice=... or ?error=... an action redirected back with.
// className places it: the default fits the app shell; sign-in pages pass their own.
export function FlashBanner({ className = "mx-4 mt-4 md:mx-8" }: { className?: string }) {
  const params = useSearchParams();
  const error = params.get("error");
  const notice = params.get("notice");
  if (error) {
    return (
      <p role="alert" className={cn("reveal rounded-md border border-danger/40 bg-danger/10 px-4 py-3 text-sm text-danger", className)}>
        {error}
      </p>
    );
  }
  if (notice) {
    return (
      <p role="status" className={cn("reveal rounded-md border border-accent/40 bg-accent/10 px-4 py-3 text-sm text-accent", className)}>
        {notice}
      </p>
    );
  }
  return null;
}
