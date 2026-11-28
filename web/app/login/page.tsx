import type { Metadata } from "next";
import Link from "next/link";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";

export const metadata: Metadata = { title: "Sign in · Kerdos" };

export default function LoginPage() {
  return (
    <main className="grid min-h-screen place-items-center px-4 py-10">
      <Card className="reveal grid w-full max-w-sm gap-6 p-8">
        <div className="flex items-center gap-2 text-lg font-semibold tracking-tight">
          <span aria-hidden="true" className="grid size-8 place-items-center rounded-md bg-accent text-accent-foreground">κ</span>
          Kerdos
        </div>
        <div className="grid gap-2">
          <h1 className="text-2xl font-semibold tracking-tight">Sign in</h1>
          <p className="text-muted">Invoices, reminders and a cash forecast for your business.</p>
        </div>
        <div className="grid gap-3">
          <Button asChild variant="primary">
            <Link href="/">Continue to the demo</Link>
          </Button>
          <Button disabled aria-describedby="google-note">Continue with Google</Button>
          <p id="google-note" className="text-sm text-muted">Google sign-in isn&apos;t enabled on this server yet.</p>
        </div>
      </Card>
    </main>
  );
}
