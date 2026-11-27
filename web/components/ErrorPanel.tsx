import Link from "next/link";
import { ApiError } from "@/lib/api/client";

export function ErrorPanel({ error }: { error: ApiError }) {
  const unreachable = error.status === 0;
  return (
    <main className="shell__main" role="alert" style={{ paddingTop: "2rem" }}>
      <div className="slab" style={{ maxWidth: "36rem", display: "grid", gap: "0.75rem" }}>
        <h1>{unreachable ? "Kerdos can't reach its server" : "Something went wrong"}</h1>
        <p>{error.message}</p>
        <div>
          <Link href="/" className="key key--primary">
            Try again
          </Link>
        </div>
      </div>
    </main>
  );
}

// For a server component's catch block: a panel for backend errors; anything else is a bug and
// is rethrown.
export function apiErrorView(error: unknown) {
  if (!(error instanceof ApiError)) throw error;
  return <ErrorPanel error={error} />;
}
