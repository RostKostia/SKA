"use client";

import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { Dialog, DialogClose, DialogContent, DialogTrigger } from "@/components/ui/Dialog";
import { StatusBadge } from "@/components/ui/Badge";
import { Tooltip } from "@/components/ui/Tooltip";
import type { DisplayStatus } from "@/lib/api/types";

const ORDER: DisplayStatus[] = ["unpaid", "overdue", "partially_paid", "paid"];

// Cycles the status; key={status} replays the badge entrance.
export function StatusDemo() {
  const [index, setIndex] = useState(0);
  const status = ORDER[index % ORDER.length];
  return (
    <div className="flex flex-wrap items-center gap-4">
      <StatusBadge key={status} status={status} />
      <Button size="sm" onClick={() => setIndex((i) => i + 1)}>Change status</Button>
    </div>
  );
}

export function OverlayDemo() {
  return (
    <div className="flex flex-wrap items-center gap-4">
      <Dialog>
        <DialogTrigger asChild>
          <Button>Open dialog</Button>
        </DialogTrigger>
        <DialogContent title="Cancel invoice" description="The client will no longer receive reminders for it.">
          <div className="flex justify-end gap-2">
            <DialogClose asChild>
              <Button>Keep invoice</Button>
            </DialogClose>
            <DialogClose asChild>
              <Button variant="danger">Cancel invoice</Button>
            </DialogClose>
          </div>
        </DialogContent>
      </Dialog>
      <Tooltip content="Sends a polite reminder today">
        <Button>Hover for a tooltip</Button>
      </Tooltip>
    </div>
  );
}
