"use client";

import { useId } from "react";
import { Area, AreaChart, CartesianGrid, ReferenceDot, ReferenceLine, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import type { Forecast } from "@/lib/api/types";
import { isFlat, lowPoint, niceTicks, zeroOffset } from "@/lib/chart";
import { formatDate, formatMoney } from "@/lib/format";

const axisMoney = new Intl.NumberFormat("en-IE", { style: "currency", currency: "EUR", notation: "compact", maximumFractionDigits: 1 });
const shortDate = new Intl.DateTimeFormat("en-GB", { day: "numeric", month: "short", timeZone: "UTC" });
const toShort = (iso: string) => shortDate.format(new Date(`${iso}T00:00:00Z`));

type Datum = { date: string; balance: number };

function ChartTooltip({ active, payload }: { active?: boolean; payload?: ReadonlyArray<{ payload?: Datum }> }) {
  const datum = payload?.[0]?.payload;
  if (!active || !datum) return null;
  return (
    <div className="rounded-md border border-border bg-surface-raised px-3 py-2 text-sm">
      <p className="text-muted">{formatDate(datum.date)}</p>
      <p className={datum.balance < 0 ? "font-semibold text-danger" : "font-semibold"}>{formatMoney(datum.balance, "EUR")}</p>
    </div>
  );
}

// The lowest balance sits beside the chart, not on it, so no label ever collides with the line or the axes.
function LowestBalance({ low }: { low: { date: string; balance_cents: number } }) {
  const negative = low.balance_cents < 0;
  return (
    <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
      <p className="text-sm text-muted">Lowest balance</p>
      <p className={negative ? "tabular text-2xl font-semibold text-danger" : "tabular text-2xl font-semibold"}>
        {formatMoney(low.balance_cents, "EUR")}
      </p>
      <p className="text-sm text-muted">on {formatDate(low.date)}</p>
    </div>
  );
}

// Balance per day over the horizon: a zero line, negative stretches in the danger colour, and the lowest point marked by a dot.
export function ForecastChart({ forecast }: { forecast: Forecast }) {
  // SVG gradient ids are page-global, so each chart needs its own.
  const id = useId().replace(/[^a-zA-Z0-9_-]/g, "");
  if (forecast.points.length === 0) return <p className="text-muted">No forecast data yet.</p>;

  const data: Datum[] = forecast.points.map((p) => ({ date: p.date, balance: p.balance_cents }));
  const offset = zeroOffset(data.map((d) => d.balance));
  const flat = isFlat(data.map((d) => d.balance));
  const flatColor = data[0].balance < 0 ? "var(--danger)" : "var(--accent)";
  const low = lowPoint(forecast);
  const ticks = niceTicks(Math.min(...data.map((d) => d.balance)), Math.max(...data.map((d) => d.balance)));

  return (
    <div className="grid gap-4">
      {low ? <LowestBalance low={low} /> : null}
      <div role="img" aria-label={`Balance forecast for the next ${forecast.horizon_days} days. ${forecast.headline}`}>
        <ResponsiveContainer width="100%" height={300}>
          <AreaChart data={data} margin={{ top: 16, right: 16, bottom: 0, left: 0 }}>
            <defs>
              <linearGradient id={`${id}-stroke`} x1="0" y1="0" x2="0" y2="1">
                <stop offset={offset} stopColor="var(--accent)" />
                <stop offset={offset} stopColor="var(--danger)" />
              </linearGradient>
              <linearGradient id={`${id}-fill`} x1="0" y1="0" x2="0" y2="1">
                <stop offset={offset} stopColor="var(--accent)" stopOpacity={0.22} />
                <stop offset={offset} stopColor="var(--danger)" stopOpacity={0.32} />
              </linearGradient>
            </defs>
            <CartesianGrid stroke="var(--border)" strokeDasharray="2 4" vertical={false} />
            <XAxis
              dataKey="date"
              tickFormatter={toShort}
              tick={{ fill: "var(--muted)", fontSize: 12 }}
              axisLine={{ stroke: "var(--border)" }}
              tickLine={false}
              minTickGap={24}
            />
            <YAxis
              width={64}
              tickFormatter={(cents: number) => axisMoney.format(cents / 100)}
              tick={{ fill: "var(--muted)", fontSize: 12 }}
              axisLine={false}
              tickLine={false}
              ticks={ticks}
              domain={[ticks[0], ticks[ticks.length - 1]]}
            />
            <Tooltip content={(props) => <ChartTooltip {...props} />} cursor={{ stroke: "var(--muted)", strokeDasharray: "3 3" }} />
            <ReferenceLine y={0} stroke="var(--muted)" strokeDasharray="5 4" />
            <Area
              className="draw-in"
              type="monotone"
              dataKey="balance"
              baseValue={0}
              stroke={flat ? flatColor : `url(#${id}-stroke)`}
              strokeWidth={2.5}
              fill={`url(#${id}-fill)`}
              isAnimationActive={false}
            />
            {low ? (
              <ReferenceDot
                x={low.date}
                y={low.balance_cents}
                r={5}
                fill={low.balance_cents < 0 ? "var(--danger)" : "var(--accent)"}
                stroke="var(--background)"
                strokeWidth={2}
              />
            ) : null}
          </AreaChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}
