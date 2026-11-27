export function formatMoney(cents: number, currency: string): string {
  return new Intl.NumberFormat("en-IE", { style: "currency", currency }).format(cents / 100);
}

// A bare number for a table column that has its own currency column.
export function formatAmount(cents: number): string {
  return new Intl.NumberFormat("en-IE", { minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(cents / 100);
}

// For sentences like "you'll be €800 negative": no cents when the amount is whole euros.
export function formatMoneyCompact(cents: number, currency: string): string {
  const digits = cents % 100 === 0 ? 0 : 2;
  return new Intl.NumberFormat("en-IE", {
    style: "currency",
    currency,
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  }).format(cents / 100);
}

// Date-only strings are formatted as UTC so the day never shifts with the viewer's timezone.
export function formatDate(isoDate: string): string {
  const [year, month, day] = isoDate.split("-").map(Number);
  return new Intl.DateTimeFormat("en-GB", {
    day: "numeric",
    month: "short",
    year: "numeric",
    timeZone: "UTC",
  }).format(new Date(Date.UTC(year, month - 1, day)));
}
