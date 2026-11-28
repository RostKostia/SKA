import { describe, expect, it } from "vitest";
import type { Forecast, ForecastPoint } from "@/lib/api/types";
import { isFlat, lowPoint, niceTicks, zeroOffset } from "./chart";

const point = (day: number, balance_cents: number): ForecastPoint => ({
  day,
  date: `2026-12-${String(day).padStart(2, "0")}`,
  balance_cents,
});

function forecast(points: ForecastPoint[], shortfall: Forecast["shortfall"] = null): Forecast {
  return {
    today: "2026-11-28",
    horizon_days: 30,
    start_balance_cents: 0,
    points,
    events: [],
    shortfall,
    headline: "",
    method: "",
  };
}

describe("zeroOffset", () => {
  it("is 1 when the balance never goes below zero", () => {
    expect(zeroOffset([5, 10, 20])).toBe(1);
  });

  it("is 0 when the balance never rises above zero", () => {
    expect(zeroOffset([-5, -10])).toBe(0);
  });

  it("is the share of the range above zero when it crosses", () => {
    expect(zeroOffset([10, -10])).toBe(0.5);
  });

  it("is 1 for an empty series", () => {
    expect(zeroOffset([])).toBe(1);
  });
});

describe("lowPoint", () => {
  const points = [point(1, 500), point(2, -200), point(3, 100)];

  it("uses the shortfall's lowest day when there is one", () => {
    const shortfall = { first_negative_day: 2, lowest_day: 2, lowest_balance_cents: -200, shortfall_cents: 200 };
    expect(lowPoint(forecast(points, shortfall))).toEqual({ date: "2026-12-02", balance_cents: -200 });
  });

  it("falls back to the minimum point when there is no shortfall", () => {
    expect(lowPoint(forecast([point(1, 500), point(2, 300), point(3, 900)]))).toEqual({ date: "2026-12-02", balance_cents: 300 });
  });

  it("is null when there are no points", () => {
    expect(lowPoint(forecast([]))).toBeNull();
  });
});

describe("niceTicks", () => {
  it("includes zero and evenly spaced round values around a series that crosses it", () => {
    expect(niceTicks(-80_000, 130_000, 6)).toEqual([-100_000, -50_000, 0, 50_000, 100_000, 150_000]);
  });

  it("starts at zero for a series that stays positive", () => {
    expect(niceTicks(120_000, 240_000, 6)).toEqual([0, 50_000, 100_000, 150_000, 200_000, 250_000]);
  });

  it("still spans a visible range when every value is zero", () => {
    expect(niceTicks(0, 0)).toEqual([0, 100_000]);
  });
});

describe("isFlat", () => {
  it("is true when every balance is the same, which a gradient stroke cannot paint", () => {
    expect(isFlat([500, 500, 500])).toBe(true);
  });

  it("is false when the balance changes or there is no data", () => {
    expect(isFlat([500, 501])).toBe(false);
    expect(isFlat([])).toBe(false);
  });
});
