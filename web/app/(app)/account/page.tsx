import type { Metadata } from "next";
import Link from "next/link";
import { Suspense } from "react";
import { apiErrorView } from "@/components/ErrorPanel";
import { PageFallback } from "@/components/PageFallback";
import { PageHeader } from "@/components/PageHeader";
import { Badge } from "@/components/ui/Badge";
import { Card } from "@/components/ui/Card";
import { getApi } from "@/lib/api";
import { initials } from "@/lib/account";

export const metadata: Metadata = { title: "Account · Kerdos" };

async function AccountPageContent() {
  let user;
  try {
    user = await getApi().getMe();
  } catch (error) {
    return apiErrorView(error);
  }

  return (
    <div className="grid max-w-xl grid-cols-[minmax(0,1fr)] gap-6">
      <PageHeader title="Account" />
      <Card className="reveal grid gap-5" style={{ "--i": 1 } as React.CSSProperties}>
        <div className="flex items-center gap-4">
          <span aria-hidden="true" className="grid size-14 shrink-0 place-items-center rounded-full bg-surface-raised text-lg font-semibold">
            {initials(user.name)}
          </span>
          <div className="grid min-w-0 gap-1">
            <p className="truncate text-lg font-semibold">{user.name}</p>
            <Badge tone="info" className="w-fit">{user.role}</Badge>
          </div>
        </div>
        <dl className="grid gap-3 text-sm">
          <div className="grid gap-1">
            <dt className="text-muted">Name</dt>
            <dd className="break-words">{user.name}</dd>
          </div>
          <div className="grid gap-1">
            <dt className="text-muted">Email</dt>
            <dd className="break-all">{user.email}</dd>
          </div>
          <div className="grid gap-1">
            <dt className="text-muted">Role</dt>
            <dd className="capitalize">{user.role}</dd>
          </div>
        </dl>
        <Link href="/login" className="w-fit text-sm text-accent underline-offset-4 hover:underline">
          Sign-in options
        </Link>
      </Card>
    </div>
  );
}

export default function AccountPage() {
  return (
    <Suspense fallback={<PageFallback />}>
      <AccountPageContent />
    </Suspense>
  );
}
