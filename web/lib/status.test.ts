import { describe, expect, it } from "vitest";
import type { DisplayStatus } from "@/lib/api/types";
import { statusMeta, type Tone } from "./status";

const EXPECTED: [DisplayStatus, string, Tone][] = [
  ["draft", "Draft", "neutral"],
  ["sent", "Sent", "neutral"],
  ["cancelled", "Cancelled", "neutral"],
  ["paid", "Paid", "success"],
  ["financed", "Financed", "info"],
  ["overdue", "Overdue", "danger"],
  ["partially_paid", "Part paid", "warning"],
  ["unpaid", "Unpaid", "warning"],
];

describe("statusMeta", () => {
  it.each(EXPECTED)("%s is labelled %s with tone %s", (status, label, tone) => {
    expect(statusMeta(status)).toEqual({ label, tone });
  });
});
