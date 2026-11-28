"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { NAV_ITEMS, TEAM_ITEM } from "@/lib/nav";
import { cn } from "@/lib/utils";

// owner adds the Team page, which only the owner can open.
export function NavLinks({ onNavigate, owner = false }: { onNavigate?: () => void; owner?: boolean }) {
  const pathname = usePathname();
  const items = owner ? [...NAV_ITEMS, TEAM_ITEM] : NAV_ITEMS;
  return (
    <ul className="grid gap-1">
      {items.map(({ href, label, icon: Icon }) => {
        const current = href === "/" ? pathname === "/" : pathname.startsWith(href);
        return (
          <li key={href}>
            <Link
              href={href}
              aria-current={current ? "page" : undefined}
              onClick={onNavigate}
              className={cn(
                "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium text-muted transition-colors hover:bg-surface-raised hover:text-foreground",
                current && "bg-surface-raised text-foreground shadow-[inset_2px_0_0_var(--accent)]",
              )}
            >
              <Icon className="size-4" aria-hidden="true" />
              {label}
            </Link>
          </li>
        );
      })}
    </ul>
  );
}
