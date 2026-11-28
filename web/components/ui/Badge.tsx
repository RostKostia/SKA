import type { HTMLAttributes } from "react";
import type { DisplayStatus } from "@/lib/api/types";
import { statusMeta, type Tone } from "@/lib/status";
import { cn } from "@/lib/utils";

const TONES: Record<Tone, string> = {
  neutral: "border-border text-muted",
  success: "border-success/40 bg-success/10 text-success",
  warning: "border-warning/40 bg-warning/10 text-warning",
  danger: "border-danger/40 bg-danger/10 text-danger",
  info: "border-info/40 bg-info/10 text-info",
};

export function Badge({ tone = "neutral", className, ...rest }: HTMLAttributes<HTMLSpanElement> & { tone?: Tone }) {
  return (
    <span
      className={cn("inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-medium", TONES[tone], className)}
      {...rest}
    />
  );
}

// Give it key={status} to replay the short entrance when the status changes.
export function StatusBadge({ status, compact = false }: { status: DisplayStatus; compact?: boolean }) {
  const { label, tone } = statusMeta(status);
  return (
    <Badge tone={tone} className={cn("animate-in fade-in zoom-in-95 duration-200", compact && "px-2 text-[0.7rem]")}>
      {label}
    </Badge>
  );
}
