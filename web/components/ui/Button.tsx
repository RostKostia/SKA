import { cva, type VariantProps } from "class-variance-authority";
import { Slot } from "radix-ui";
import type { ButtonHTMLAttributes } from "react";
import { cn } from "@/lib/utils";

const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md font-medium transition-[background-color,transform,opacity] duration-150 ease-out active:scale-[0.98] disabled:pointer-events-none disabled:opacity-50",
  {
    variants: {
      variant: {
        primary: "bg-accent text-accent-foreground hover:bg-accent/90",
        quiet: "border border-border bg-surface-raised text-foreground hover:bg-border",
        danger: "bg-danger text-danger-foreground hover:bg-danger/90",
      },
      size: { sm: "h-8 px-3 text-sm", md: "h-10 px-4 text-sm" },
    },
    defaultVariants: { variant: "quiet", size: "md" },
  },
);

type Props = ButtonHTMLAttributes<HTMLButtonElement> & VariantProps<typeof buttonVariants> & { asChild?: boolean };

export function Button({ variant, size, asChild = false, type = "button", className, ...rest }: Props) {
  const classes = cn(buttonVariants({ variant, size }), className);
  return asChild ? <Slot.Root className={classes} {...rest} /> : <button type={type} className={classes} {...rest} />;
}
