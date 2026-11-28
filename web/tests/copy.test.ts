import { existsSync, readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const root = fileURLToPath(new URL("..", import.meta.url));

function sources(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) return sources(path);
    return /\.(ts|tsx)$/.test(name) ? [path] : [];
  });
}

const files = ["app", "components"].flatMap((dir) => sources(join(root, dir)));
const text = (path: string) => readFileSync(path, "utf8");

// Class names of the old limestone-and-paper look.
const OLD_CLASSES = /^(slab|slip|stamp|well|key|banner|shell|table|table-wrap|chat|cards|stack|row|tabs|toolbar|field)(__|--|$)/;

describe("copy and leftovers", () => {
  it("never uses the 'Who owes' wording in the UI", () => {
    const offenders = files.filter((f) => /who owes/i.test(text(f))).map((f) => relative(root, f));
    expect(offenders).toEqual([]);
  });

  it("does not import the removed paper, surface or stamp components", () => {
    const offenders = files
      .filter((f) => /from "@\/components\/ui\/(Paper|Surface|StatusStamp)"/.test(text(f)))
      .map((f) => relative(root, f));
    expect(offenders).toEqual([]);
  });

  it("does not use the old design's class names", () => {
    const offenders = files.filter((f) => {
      const classNames = [...text(f).matchAll(/className=(?:"([^"]*)"|\{`([^`]*)`\})/g)].flatMap((m) =>
        (m[1] ?? m[2]).split(/\s+/),
      );
      return classNames.some((c) => OLD_CLASSES.test(c));
    });
    expect(offenders.map((f) => relative(root, f))).toEqual([]);
  });

  it.each(["components/ui/Paper.tsx", "components/ui/Surface.tsx", "components/ui/StatusStamp.tsx", "app/page.module.css"])(
    "has removed %s",
    (path) => {
      expect(existsSync(join(root, path))).toBe(false);
    },
  );

  it("pins page grids to one shrinkable column so wide tables scroll instead of widening the page", () => {
    const offenders = files
      .filter((f) => f.includes(join("app", "(app)")) && /className="grid gap-6"/.test(text(f)))
      .map((f) => relative(root, f));
    expect(offenders).toEqual([]);
  });

  it("positions the table scroll wrapper so visually hidden cells cannot widen the page", () => {
    expect(text(join(root, "components", "ui", "Table.tsx"))).toMatch(/className="relative overflow-x-auto/);
  });

  it("keeps the lowest balance outside the plot instead of labelling it on the chart", () => {
    const chart = text(join(root, "components", "ForecastChart.tsx"));
    expect(chart).not.toMatch(/\slabel=\{/);
    expect(chart).toContain("Lowest balance");
  });

  it("lets long unbroken text wrap in chat bubbles and reminder templates", () => {
    for (const page of [["app", "(app)", "chat", "page.tsx"], ["app", "(app)", "reminders", "page.tsx"]]) {
      expect(text(join(root, ...page))).toContain("[overflow-wrap:anywhere]");
    }
  });

  it("truncates long names in an inner span, because table cells ignore max-width", () => {
    const offenders = files.filter((f) => /<(Td|Th)[^>]*max-w-/.test(text(f))).map((f) => relative(root, f));
    expect(offenders).toEqual([]);
  });
});
