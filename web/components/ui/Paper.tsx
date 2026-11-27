import type { CSSProperties, ReactNode } from "react";

type Props = {
  children: ReactNode;
  // Tear-off stub, usually a StatusStamp.
  stub?: ReactNode;
  // Slight rotation in degrees, so a stack of slips looks handled.
  tilt?: number;
  // Slides in from the left, for a freshly created invoice.
  isNew?: boolean;
};

export function Paper({ children, stub, tilt = 0, isNew = false }: Props) {
  return (
    <article
      className={isNew ? "slip slip--new" : "slip"}
      style={{ "--tilt": `${tilt}deg` } as CSSProperties}
    >
      <div className="slip__main">{children}</div>
      {stub ? <div className="slip__stub">{stub}</div> : null}
    </article>
  );
}
