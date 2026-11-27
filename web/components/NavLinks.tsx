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

export function NavLinks() {
  const pathname = usePathname();
  return (
    <ul>
      {ITEMS.map(({ href, label }) => {
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
