import { describe, expect, it } from "vitest";
import { fold } from "./search";

describe("fold", () => {
  it("ignores case and accents", () => {
    expect(fold("Élena's BAKERY")).toBe("elena's bakery");
  });

  it("matches Greek without accents and with either sigma", () => {
    expect(fold("Μαρία Σάββα").includes(fold("μαρια σαββα"))).toBe(true);
    expect(fold("ΚΩΣΤΑΣ")).toBe(fold("κώστασ"));
  });
});
