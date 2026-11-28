import type { DisplayStatus } from "@/lib/api/types";

export type Tone = "neutral" | "success" | "warning" | "danger" | "info";

const META: Record<DisplayStatus, { label: string; tone: Tone }> = {
  draft: { label: "Draft", tone: "neutral" },
  sent: { label: "Sent", tone: "neutral" },
  cancelled: { label: "Cancelled", tone: "neutral" },
  paid: { label: "Paid", tone: "success" },
  financed: { label: "Financed", tone: "info" },
  overdue: { label: "Overdue", tone: "danger" },
  partially_paid: { label: "Part paid", tone: "warning" },
  unpaid: { label: "Unpaid", tone: "warning" },
};

export function statusMeta(status: DisplayStatus): { label: string; tone: Tone } {
  return META[status];
}
