import Link from "next/link";
import { ApiError } from "@/lib/api/client";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";

export function ErrorPanel({ error }: { error: ApiError }) {
  const unreachable = error.status === 0;
  return (
    <Card role="alert" className="grid max-w-xl gap-3">
      <h1 className="text-xl font-semibold tracking-tight">
        {unreachable ? "Kerdos can't reach its server" : "Something went wrong"}
      </h1>
      <p className="text-muted">{error.message}</p>
      <div>
        <Button asChild variant="primary">
          <Link href="/">Try again</Link>
        </Button>
      </div>
    </Card>
  );
}

// For a server component's catch block: a panel for backend errors; anything else is a bug and
// is rethrown.
export function apiErrorView(error: unknown) {
  if (!(error instanceof ApiError)) throw error;
  return <ErrorPanel error={error} />;
}
