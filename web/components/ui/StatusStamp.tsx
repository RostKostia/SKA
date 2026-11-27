import type { DisplayStatus } from "@/lib/api/types";

const LABELS: Record<DisplayStatus, string> = {
  draft: "Draft",
  sent: "Sent",
  cancelled: "Cancelled",
  paid: "Paid",
  financed: "Financed",
  overdue: "Overdue",
  partially_paid: "Part paid",
  unpaid: "Unpaid",
};

// The stamp thumps in when it mounts. Give it key={status} to replay it when the status changes.
// `compact` is a small, still version for table rows.
export function StatusStamp({ status, compact = false }: { status: DisplayStatus; compact?: boolean }) {
  return <span className={`stamp stamp--${status}${compact ? " stamp--sm" : ""}`}>{LABELS[status] ?? status}</span>;
}
