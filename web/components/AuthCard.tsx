import type { ReactNode } from "react";
import { Card } from "@/components/ui/Card";

// The card every sign-in screen sits in: the brand, a title and a short line under it.
export function AuthCard({ title, description, children }: { title: string; description?: ReactNode; children: ReactNode }) {
  return (
    <Card className="reveal grid w-full max-w-sm gap-6 p-8">
      <div className="flex items-center gap-2 text-lg font-semibold tracking-tight">
        <span aria-hidden="true" className="grid size-8 place-items-center rounded-md bg-accent text-accent-foreground">κ</span>
        Kerdos
      </div>
      <div className="grid gap-2">
        <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
        {description ? <p className="text-muted">{description}</p> : null}
      </div>
      {children}
    </Card>
  );
}

export function AuthCardFallback() {
  return <Card className="h-96 w-full max-w-sm animate-pulse" aria-busy="true" />;
}

// "or with your email" between the Google button and the form.
export function OrDivider({ children }: { children: ReactNode }) {
  return (
    <div className="flex items-center gap-3 text-xs text-muted">
      <span className="h-px flex-1 bg-border" />
      {children}
      <span className="h-px flex-1 bg-border" />
    </div>
  );
}

export function Field({ label, hint, children }: { label: string; hint?: string; children: ReactNode }) {
  return (
    <label className="grid gap-1.5 text-sm font-medium">
      {label}
      {children}
      {hint ? <span className="text-xs font-normal text-muted">{hint}</span> : null}
    </label>
  );
}
