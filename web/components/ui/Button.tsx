import type { ButtonHTMLAttributes } from "react";

type Props = ButtonHTMLAttributes<HTMLButtonElement> & { variant?: "primary" | "quiet" };

export function Button({ variant = "quiet", className, type = "button", ...rest }: Props) {
  const classes = ["key", variant === "primary" && "key--primary", className].filter(Boolean);
  return <button type={type} className={classes.join(" ")} {...rest} />;
}
