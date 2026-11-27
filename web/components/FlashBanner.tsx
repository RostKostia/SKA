"use client";

import { useSearchParams } from "next/navigation";

// Shows the ?notice=... or ?error=... an action redirected back with.
export function FlashBanner() {
  const params = useSearchParams();
  const error = params.get("error");
  const notice = params.get("notice");
  if (error) return <p role="alert" className="banner banner--error">{error}</p>;
  if (notice) return <p role="status" className="banner">{notice}</p>;
  return null;
}
