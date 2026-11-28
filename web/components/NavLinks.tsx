"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

const ITEMS = [
  { href: "/", label: "Overview" },
  { href: "/chat", label: "Chat" },
  { href: "/invoices", label: "Invoices" },
  { href: "/debtors", label: "Debtors" },
  { href: "/forecast", label: "Forecast" },
  { href: "/reminders", label: "Reminders" },
];

// owner adds the Team page, which only the owner can open.
export function NavLinks({ owner = false }: { owner?: boolean }) {
  const pathname = usePathname();
  const items = [...ITEMS, ...(owner ? [{ href: "/team", label: "Team" }] : []), { href: "/account", label: "Account" }];
  return (
    <ul>
      {items.map(({ href, label }) => {
        const current = href === "/" ? pathname === "/" : pathname.startsWith(href);
        return (
          <li key={href}>
            <Link href={href} aria-current={current ? "page" : undefined}>
              {label}
            </Link>
          </li>
        );
      })}
    </ul>
  );
}
