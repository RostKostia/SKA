"use client";

import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Paper } from "@/components/ui/Paper";
import { StatusStamp } from "@/components/ui/StatusStamp";
import type { DisplayStatus } from "@/lib/api/types";

const ORDER: DisplayStatus[] = ["paid", "overdue", "financed", "unpaid"];

export function StampDemo() {
  const [index, setIndex] = useState(0);
  const status = ORDER[index];
  return (
    <div style={{ display: "grid", gap: "1rem" }}>
      <Paper tilt={-0.4} stub={<StatusStamp key={status} status={status} />}>
        <h3>INV-1041</h3>
        <p>Andreas Electric (sample)</p>
        <p style={{ fontSize: "1.75rem", fontWeight: 600 }}>€380.00</p>
        <p>Due 6 Dec 2026</p>
      </Paper>
      <div>
        <Button variant="primary" onClick={() => setIndex((index + 1) % ORDER.length)}>
          Stamp the next status
        </Button>
      </div>
      <p className="muted" aria-live="polite">
        Status: {status.replace("_", " ")}
      </p>
    </div>
  );
}

export function LineDemo() {
  const [plays, setPlays] = useState(0);
  return (
    <div style={{ display: "grid", gap: "1rem", maxWidth: "34rem" }}>
      <svg viewBox="0 0 400 120" role="img" aria-label="Sample balance line dipping below zero and recovering">
        <line x1="0" y1="80" x2="400" y2="80" stroke="var(--ink-muted)" strokeDasharray="4 5" />
        <path
          key={plays}
          className="draw-line"
          pathLength={1}
          d="M0 40 C60 30 90 60 140 70 S220 110 260 95 S340 40 400 30"
          fill="none"
          stroke="var(--accent)"
          strokeWidth="3"
          strokeLinecap="round"
        />
      </svg>
      <div>
        <Button onClick={() => setPlays(plays + 1)}>Draw again</Button>
      </div>
    </div>
  );
}
