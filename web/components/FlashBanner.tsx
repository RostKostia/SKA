"use client";

import { useSearchParams } from "next/navigation";

// Shows the ?notice=... or ?error=... an action redirected back with.
export function FlashBanner() {
  const params = useSearchParams();
  const error = params.get("error");
  const notice = params.get("notice");
  if (error) {
    return (
      <p role="alert" className="reveal mx-4 mt-4 rounded-md border border-danger/40 bg-danger/10 px-4 py-3 text-sm text-danger md:mx-8">
        {error}
      </p>
    );
  }
  if (notice) {
    return (
      <p role="status" className="reveal mx-4 mt-4 rounded-md border border-accent/40 bg-accent/10 px-4 py-3 text-sm text-accent md:mx-8">
        {notice}
      </p>
    );
  }
  return null;
}
