import { describe, expect, it } from "vitest";
import { initials } from "./account";

describe("initials", () => {
  it("takes the first letters of the first two words", () => {
    expect(initials("Andreas Kostas")).toBe("AK");
    expect(initials("Maria Anna Petrou")).toBe("MA");
  });

  it("uses one letter for a single word, upper-cased", () => {
    expect(initials("andreas")).toBe("A");
  });

  it("falls back to a question mark for empty or blank names", () => {
    expect(initials("  ")).toBe("?");
    expect(initials("")).toBe("?");
  });
});
