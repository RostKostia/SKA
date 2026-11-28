import { Bell, FileText, LayoutDashboard, MessageSquare, TrendingUp, Users, type LucideIcon } from "lucide-react";

export const ACCOUNT_HREF = "/account";

export const NAV_ITEMS: { href: string; label: string; icon: LucideIcon }[] = [
  { href: "/", label: "Overview", icon: LayoutDashboard },
  { href: "/chat", label: "Chat", icon: MessageSquare },
  { href: "/invoices", label: "Invoices", icon: FileText },
  { href: "/debtors", label: "Receivables", icon: Users },
  { href: "/forecast", label: "Forecast", icon: TrendingUp },
  { href: "/reminders", label: "Reminders", icon: Bell },
];
