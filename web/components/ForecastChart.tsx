import type { Forecast } from "@/lib/api/types";
import { formatDate, formatMoneyCompact } from "@/lib/format";

const W = 720;
const H = 260;
const PAD = { top: 24, right: 24, bottom: 36, left: 72 };

// Balance per day over the horizon, with the zero line and the lowest point marked.
export function ForecastChart({ forecast }: { forecast: Forecast }) {
  const points = forecast.points;
  if (points.length === 0) return null;
  const balances = points.map((p) => p.balance_cents);
  const max = Math.max(0, ...balances);
  const min = Math.min(0, ...balances);
  const span = max - min || 1;
  const last = points.length - 1;
  const x = (i: number) => PAD.left + (i / Math.max(1, last)) * (W - PAD.left - PAD.right);
  const y = (cents: number) => PAD.top + ((max - cents) / span) * (H - PAD.top - PAD.bottom);

  const path = points.map((p, i) => `${i ? "L" : "M"}${x(i).toFixed(1)} ${y(p.balance_cents).toFixed(1)}`).join(" ");
  const low = forecast.shortfall ? points.findIndex((p) => p.day === forecast.shortfall!.lowest_day) : -1;
  const ticks = [0, Math.round(last / 3), Math.round((last * 2) / 3), last];

  return (
    <svg viewBox={`0 0 ${W} ${H}`} role="img" className="chart" aria-label={`Balance forecast for the next ${forecast.horizon_days} days. ${forecast.headline}`}>
      <line x1={PAD.left} x2={W - PAD.right} y1={y(0)} y2={y(0)} stroke="var(--ink-muted)" strokeDasharray="4 5" />
      <text x={PAD.left - 8} y={y(0) + 5} textAnchor="end" fontSize="14" fill="var(--ink-muted)">€0</text>
      {max > 0 ? <text x={PAD.left - 8} y={y(max) + 5} textAnchor="end" fontSize="14" fill="var(--ink-muted)">{formatMoneyCompact(max, "EUR")}</text> : null}
      {min < 0 ? <text x={PAD.left - 8} y={y(min) + 5} textAnchor="end" fontSize="14" fill="var(--ink-muted)">{formatMoneyCompact(min, "EUR")}</text> : null}
      <path d={path} pathLength={1} className="draw-line" fill="none" stroke="var(--accent)" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round" />
      {low >= 0 ? (
        <g>
          <circle cx={x(low)} cy={y(points[low].balance_cents)} r="6" fill="var(--stamp-red)" />
          <text x={x(low) + 12} y={y(points[low].balance_cents) + 5} fontSize="15" fontWeight="700" fill="var(--stamp-red)">
            {formatMoneyCompact(points[low].balance_cents, "EUR")}
          </text>
        </g>
      ) : null}
      {ticks.map((i) => (
        <text key={i} x={x(i)} y={H - 10} textAnchor={i === 0 ? "start" : i === last ? "end" : "middle"} fontSize="14" fill="var(--ink-muted)">
          {formatDate(points[i].date)}
        </text>
      ))}
    </svg>
  );
}
