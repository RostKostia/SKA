import type { SelectHTMLAttributes } from "react";
import { cn } from "@/lib/utils";

// A native select, styled like Input, so forms keep working without JavaScript.
export function Select({ className, ...rest }: SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <select
      className={cn(
        "h-10 w-full rounded-md border border-border bg-background px-3 text-sm text-foreground transition-colors focus-visible:border-accent",
        className,
      )}
      {...rest}
    />
  );
}
