"use client";

import { Dialog as DialogPrimitive } from "radix-ui";
import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

export const Dialog = DialogPrimitive.Root;
export const DialogTrigger = DialogPrimitive.Trigger;
export const DialogClose = DialogPrimitive.Close;

type ContentProps = {
  title: string;
  description?: string;
  children: ReactNode;
  className?: string;
  side?: "center" | "left";
};

export function DialogContent({ title, description, children, className, side = "center" }: ContentProps) {
  return (
    <DialogPrimitive.Portal>
      <DialogPrimitive.Overlay className="fixed inset-0 z-40 bg-black/60 animate-in fade-in duration-200" />
      <DialogPrimitive.Content
        className={cn(
          "fixed z-50 border border-border bg-surface p-5 animate-in duration-200",
          side === "center"
            ? "left-1/2 top-1/2 w-[min(32rem,calc(100vw-2rem))] -translate-x-1/2 -translate-y-1/2 rounded-lg fade-in zoom-in-95"
            : "inset-y-0 left-0 w-72 slide-in-from-left",
          className,
        )}
      >
        <DialogPrimitive.Title className="text-base font-semibold">{title}</DialogPrimitive.Title>
        <DialogPrimitive.Description className={description ? "mt-1 text-sm text-muted" : "sr-only"}>
          {description ?? title}
        </DialogPrimitive.Description>
        <div className="mt-4">{children}</div>
      </DialogPrimitive.Content>
    </DialogPrimitive.Portal>
  );
}
