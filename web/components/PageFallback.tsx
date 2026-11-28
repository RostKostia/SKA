import { Skeleton } from "@/components/ui/Skeleton";

// Shown while a page waits for the backend, so navigating between pages is instant.
export function PageFallback() {
  return (
    <div className="grid gap-4" aria-busy="true">
      <Skeleton className="h-8 w-48" />
      <div className="grid gap-4 sm:grid-cols-3">
        <Skeleton className="h-24" />
        <Skeleton className="h-24" />
        <Skeleton className="h-24" />
      </div>
      <Skeleton className="h-64" />
      <span className="sr-only">Loading…</span>
    </div>
  );
}
