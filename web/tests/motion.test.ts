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
});
