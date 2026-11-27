import type { HTMLAttributes } from "react";

type Props = HTMLAttributes<HTMLDivElement> & { inset?: boolean };

// A raised slab, or with `inset` a well pressed into the stone.
export function Surface({ inset = false, className, ...rest }: Props) {
  const classes = [inset ? "well" : "slab", className].filter(Boolean);
  return <div className={classes.join(" ")} {...rest} />;
}
