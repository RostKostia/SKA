import type { Forecast } from "@/lib/api/types";

// Where zero sits in a vertical gradient that runs from the series' maximum (0) to its
// minimum (1). Everything below the stop is a negative balance.
export function zeroOffset(balances: number[]): number {
  if (balances.length === 0) return 1;
  const max = Math.max(...balances);
  const min = Math.min(...balances);
  if (min >= 0) return 1;
  if (max <= 0) return 0;
  return max / (max - min);
}

// A flat line has a zero-height bounding box, where an SVG gradient stroke paints nothing.
export function isFlat(balances: number[]): boolean {
  return balances.length > 0 && balances.every((b) => b === balances[0]);
}

export function lowPoint(f: Forecast): { date: string; balance_cents: number } | null {
  if (f.points.length === 0) return null;
  const lowest = f.shortfall
    ? f.points.find((p) => p.day === f.shortfall!.lowest_day)
    : undefined;
  const point = lowest ?? f.points.reduce((a, b) => (b.balance_cents < a.balance_cents ? b : a));
  return { date: point.date, balance_cents: point.balance_cents };
}

// Round, evenly spaced y-axis values (in cents) that always include zero and cover [min, max].
export function niceTicks(min: number, max: number, target = 6): number[] {
  const lo = Math.min(0, min);
  const hi = Math.max(0, max);
  if (lo === hi) return [0, 100_000]; // all zero: keep a visible axis range
  const rough = (hi - lo) / (target - 1);
  const magnitude = 10 ** Math.floor(Math.log10(rough));
  const step = Math.max(1, [1, 2, 5, 10].map((m) => m * magnitude).find((s) => s >= rough) ?? 10 * magnitude);
  const ticks: number[] = [];
  for (let v = Math.floor(lo / step) * step; v <= Math.ceil(hi / step) * step; v += step) ticks.push(v);
  return ticks;
}
