import type { Metadata } from "next";
import { ForecastChart } from "@/components/ForecastChart";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { Skeleton } from "@/components/ui/Skeleton";
import { Table, TBody, Td, Th, THead, Tr } from "@/components/ui/Table";
import type { Forecast } from "@/lib/api/types";
import type { Tone } from "@/lib/status";
import { OverlayDemo, StatusDemo } from "./Demos";

export const metadata: Metadata = { title: "Design kit · Kerdos" };

const SWATCHES = [
  ["--background", "Background", "#0b1220"],
  ["--surface", "Surface", "#111a2e"],
  ["--surface-raised", "Raised", "#18233b"],
  ["--border", "Border", "#25324d"],
  ["--foreground", "Foreground", "#e6ecf7"],
  ["--muted", "Muted", "#93a4c3"],
  ["--accent", "Accent", "#2dd4bf"],
  ["--success", "Success", "#4ade80"],
  ["--warning", "Warning", "#fbbf24"],
  ["--danger", "Danger", "#f87171"],
  ["--info", "Info", "#60a5fa"],
] as const;

const TONES: Tone[] = ["neutral", "success", "warning", "danger", "info"];

function fixture(balances: number[], headline: string): Forecast {
  const points = balances.map((balance_cents, day) => ({
    day,
    date: `2026-12-${String(day + 1).padStart(2, "0")}`,
    balance_cents,
  }));
  const lowest = points.reduce((a, b) => (b.balance_cents < a.balance_cents ? b : a), points[0]);
  const negative = points.find((p) => p.balance_cents < 0);
  return {
    today: "2026-12-01",
    horizon_days: balances.length,
    start_balance_cents: balances[0] ?? 0,
    points,
    events: [],
    shortfall:
      lowest && negative
        ? { first_negative_day: negative.day, lowest_day: lowest.day, lowest_balance_cents: lowest.balance_cents, shortfall_cents: -lowest.balance_cents }
        : null,
    headline,
    method: "Fixture data for the design kit.",
  };
}

const FIXTURES: [string, Forecast][] = [
  ["Forecast that never goes negative", fixture([120_000, 150_000, 140_000, 180_000, 210_000, 190_000, 240_000], "You stay above zero.")],
  ["Forecast below zero throughout", fixture([-40_000, -90_000, -65_000, -120_000, -80_000, -30_000, -10_000], "You start and stay below zero.")],
  ["Forecast that crosses zero", fixture([90_000, 40_000, -30_000, -80_000, -20_000, 50_000, 130_000], "In 3 days you'll be €800 negative.")],
  ["Forecast with a flat balance", fixture([50_000, 50_000, 50_000, 50_000, 50_000, 50_000, 50_000], "Nothing changes this week.")],
  ["Forecast with no data", fixture([], "No forecast yet.")],
];

export default function DesignKit() {
  return (
    <main className="mx-auto grid max-w-5xl gap-6 px-4 py-8 pb-16">
      <header className="reveal grid gap-2">
        <h1 className="text-3xl font-semibold tracking-tight">Kerdos design kit</h1>
        <p className="max-w-prose text-muted">
          A dark navy control room: hairline borders, one bright accent, tabular numerals. Motion only answers
          something you did.
        </p>
      </header>

      <Card aria-labelledby="palette" className="reveal" style={{ "--i": 1 } as React.CSSProperties}>
        <CardHeader>
          <CardTitle id="palette">Palette</CardTitle>
        </CardHeader>
        <ul className="grid list-none grid-cols-[repeat(auto-fill,minmax(8rem,1fr))] gap-3 p-0">
          {SWATCHES.map(([token, name, hex]) => (
            <li key={token} className="grid gap-1 text-sm">
              <div className="h-14 rounded-md border border-border" style={{ background: `var(${token})` }} />
              <span className="font-medium">{name}</span>
              <span className="text-muted">{hex}</span>
            </li>
          ))}
        </ul>
      </Card>

      <Card aria-labelledby="type" className="reveal" style={{ "--i": 2 } as React.CSSProperties}>
        <CardHeader>
          <CardTitle id="type">Type</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-2xl font-semibold tracking-tight">One sans family for everything you read</p>
          <p>Μαρία Σάββα, Кирилл and Kostas are all set in the same family, so every client name looks right.</p>
          <p className="tabular text-lg">€1,234,567.89 · 28 Nov 2026</p>
        </CardContent>
      </Card>

      <Card aria-labelledby="controls" className="reveal" style={{ "--i": 3 } as React.CSSProperties}>
        <CardHeader>
          <CardTitle id="controls">Buttons and fields</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex flex-wrap gap-3">
            <Button>Remind now</Button>
            <Button variant="primary">Send invoice</Button>
            <Button variant="danger">Cancel invoice</Button>
            <Button size="sm">Small</Button>
            <Button disabled>Sending</Button>
          </div>
          <div className="max-w-sm">
            <label htmlFor="demo-field" className="sr-only">Invoice message</label>
            <Input id="demo-field" placeholder="Invoice Kostas, 3 hours at €40" />
          </div>
          <OverlayDemo />
        </CardContent>
      </Card>

      <Card aria-labelledby="badges" className="reveal" style={{ "--i": 4 } as React.CSSProperties}>
        <CardHeader>
          <CardTitle id="badges">Badges</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex flex-wrap gap-2">
            {TONES.map((tone) => (
              <Badge key={tone} tone={tone}>{tone}</Badge>
            ))}
          </div>
          <StatusDemo />
        </CardContent>
      </Card>

      <section aria-labelledby="table" className="reveal grid gap-3" style={{ "--i": 5 } as React.CSSProperties}>
        <h2 id="table" className="text-base font-semibold">Table</h2>
        <Table caption="Sample invoices">
          <THead>
            <Tr>
              <Th>Invoice</Th>
              <Th>Client</Th>
              <Th className="text-right">Amount</Th>
            </Tr>
          </THead>
          <TBody>
            <Tr>
              <Td>KRD-2026-0001</Td>
              <Td>Μαρία Σάββα</Td>
              <Td className="text-right">€1,234,567.89</Td>
            </Tr>
            <Tr>
              <Td>KRD-2026-0002</Td>
              <Td>A client with a very long company name that must not widen the page</Td>
              <Td className="text-right">€120.00</Td>
            </Tr>
          </TBody>
        </Table>
      </section>

      <section aria-labelledby="states" className="grid gap-3">
        <h2 id="states" className="text-base font-semibold">Empty, error and shortfall states</h2>
        <Card>
          <p className="text-muted">No outstanding receivables.</p>
        </Card>
        <Card className="grid gap-3" aria-busy="true">
          <Skeleton className="h-4 w-1/3" />
          <Skeleton className="h-24 w-full" />
        </Card>
        {FIXTURES.map(([title, forecast]) => (
          <Card key={title} aria-label={title}>
            <CardHeader>
              <CardTitle>{title}</CardTitle>
            </CardHeader>
            <ForecastChart forecast={forecast} />
          </Card>
        ))}
      </section>
    </main>
  );
}
