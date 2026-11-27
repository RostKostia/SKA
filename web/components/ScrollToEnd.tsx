"use client";

import { useEffect, useRef } from "react";

// Keeps the newest chat message in view when the count changes.
export function ScrollToEnd({ count }: { count: number }) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    ref.current?.scrollIntoView({ block: "end" });
  }, [count]);
  return <div ref={ref} aria-hidden="true" />;
}
