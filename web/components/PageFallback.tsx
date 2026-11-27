// Shown while a page waits for the backend, so navigating between pages is instant.
export function PageFallback() {
  return (
    <main className="shell__main" aria-busy="true">
      <p className="muted">Loading…</p>
    </main>
  );
}
