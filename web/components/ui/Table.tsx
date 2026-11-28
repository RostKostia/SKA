import type { HTMLAttributes, TdHTMLAttributes, ThHTMLAttributes } from "react";
import { cn } from "@/lib/utils";

// The wrapper scrolls sideways so a wide table never widens the page.
export function Table({ caption, className, children, ...rest }: HTMLAttributes<HTMLTableElement> & { caption?: string }) {
  return (
    <div className="relative overflow-x-auto rounded-lg border border-border bg-surface">
      <table className={cn("w-full border-collapse text-sm tabular-nums", className)} {...rest}>
        {caption ? <caption className="sr-only">{caption}</caption> : null}
        {children}
      </table>
    </div>
  );
}

export function THead(props: HTMLAttributes<HTMLTableSectionElement>) {
  return <thead {...props} />;
}

export function TBody(props: HTMLAttributes<HTMLTableSectionElement>) {
  return <tbody {...props} />;
}

export function Tr({ className, ...rest }: HTMLAttributes<HTMLTableRowElement>) {
  return <tr className={cn("border-b border-border transition-colors last:border-0 hover:bg-surface-raised/60", className)} {...rest} />;
}

export function Th({ className, scope = "col", ...rest }: ThHTMLAttributes<HTMLTableCellElement>) {
  return <th scope={scope} className={cn("px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-muted", className)} {...rest} />;
}

export function Td({ className, ...rest }: TdHTMLAttributes<HTMLTableCellElement>) {
  return <td className={cn("px-4 py-3 align-middle", className)} {...rest} />;
}
