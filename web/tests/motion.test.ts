import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

const css = readFileSync(new URL("../app/globals.css", import.meta.url), "utf8");

describe("motion", () => {
  it("stops animations and transitions when the user prefers reduced motion", () => {
    const block = css.match(/@media \(prefers-reduced-motion: reduce\)\s*\{([\s\S]*?)\n\}/);
    expect(block, "missing prefers-reduced-motion block").not.toBeNull();
    expect(block![1]).toMatch(/animation-duration:\s*0\.01ms/);
    expect(block![1]).toMatch(/transition-duration:\s*0\.01ms/);
  });

  it("defines reveal and draw-in utilities", () => {
    expect(css).toMatch(/\.reveal/);
    expect(css).toMatch(/\.draw-in/);
  });

  it("shows the chart line fully when motion is reduced", () => {
    const block = css.match(/@media \(prefers-reduced-motion: reduce\)\s*\{([\s\S]*?)\n\}/);
    expect(block![1]).toMatch(/\.draw-in\s*\{[^}]*animation:\s*none/);
    expect(block![1]).toMatch(/\.draw-in\s*\{[^}]*clip-path:\s*none/);
  });

  it("reveals the chart by clipping, so a long line is never cut by a fixed dash length", () => {
    expect(css).not.toMatch(/stroke-dasharray/);
    expect(css).toMatch(/\.draw-in\s*\{[^}]*animation:\s*draw/);
  });
});
