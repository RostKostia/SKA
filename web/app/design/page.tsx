import type { Metadata } from "next";
import { Button } from "@/components/ui/Button";
import { Surface } from "@/components/ui/Surface";
import { LineDemo, StampDemo } from "./Demos";

export const metadata: Metadata = { title: "Design kit · Kerdos" };

const SWATCHES = [
  ["--stone", "Stone", "#dde3df"],
  ["--slab", "Slab", "#e8edea"],
  ["--paper", "Paper", "#fbfaf4"],
  ["--ink", "Ink", "#13262b"],
  ["--ink-muted", "Ink, muted", "#4b6068"],
  ["--accent", "Verdigris", "#14695f"],
  ["--stamp-blue", "Stamp blue", "#2b3a8c"],
  ["--stamp-red", "Stamp red", "#b3261e"],
] as const;

export default function DesignKit() {
  return (
    <main style={{ maxWidth: "64rem", margin: "0 auto", padding: "2rem 1rem 4rem", display: "grid", gap: "2rem" }}>
      <header>
        <h1>Kerdos design kit</h1>
        <p className="muted">
          A shop counter: limestone worktop, paper slips, rubber-stamp ink and cash-register keys.
          Motion only answers something you did.
        </p>
      </header>

      <section aria-labelledby="palette" className="slab">
        <h2 id="palette">Palette</h2>
        <ul style={{ listStyle: "none", padding: 0, margin: "1rem 0 0", display: "grid", gap: "0.75rem", gridTemplateColumns: "repeat(auto-fill, minmax(9rem, 1fr))" }}>
          {SWATCHES.map(([token, name, hex]) => (
            <li key={token}>
              <div style={{ height: "3.5rem", borderRadius: 8, background: `var(${token})`, boxShadow: "inset 0 0 0 1px rgb(19 38 43 / 0.2)" }} />
              <p style={{ fontWeight: 600 }}>{name}</p>
              <p className="muted">{hex}</p>
            </li>
          ))}
        </ul>
      </section>

      <section aria-labelledby="type" className="slab" style={{ display: "grid", gap: "0.75rem" }}>
        <h2 id="type">Type</h2>
        <h1>Commissioner for everything you click</h1>
        <p>Μαρία Σάββα, Кирилл and Kostas all set in the same family, so every client name looks right.</p>
        <p style={{ fontFamily: "var(--font-paper)", fontSize: "1.125rem", fontVariantNumeric: "tabular-nums lining-nums" }}>
          Source Serif for what is printed on the slip: €1,234.50 · 27 Nov 2026
        </p>
      </section>

      <section aria-labelledby="keys" className="slab" style={{ display: "grid", gap: "1rem" }}>
        <h2 id="keys">Keys and fields</h2>
        <div style={{ display: "flex", flexWrap: "wrap", gap: "1rem" }}>
          <Button>Remind now</Button>
          <Button variant="primary">Send invoice</Button>
          <Button disabled>Sending</Button>
        </div>
        <div style={{ maxWidth: "26rem" }}>
          <label htmlFor="demo-field" className="sr-only">Invoice message</label>
          <input id="demo-field" className="field" placeholder="Invoice Kostas, 3 hours at €40" />
        </div>
        <Surface inset>
          <p>A well is pressed into the stone. The selected nav item uses it.</p>
        </Surface>
      </section>

      <section aria-labelledby="slip" className="slab" style={{ display: "grid", gap: "1rem" }}>
        <h2 id="slip">Slip and stamp</h2>
        <StampDemo />
      </section>

      <section aria-labelledby="line" className="slab" style={{ display: "grid", gap: "1rem" }}>
        <h2 id="line">Forecast line</h2>
        <LineDemo />
      </section>
    </main>
  );
}
