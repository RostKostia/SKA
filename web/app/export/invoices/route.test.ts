import { NextRequest } from "next/server";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api/client";

// A plain fake, not vi.fn: vi.fn tracks the promises it returns and reports a rejection as unhandled.
const backend = vi.hoisted(() => ({
  calls: [] as unknown[],
  impl: (async () => []) as (filter: unknown) => Promise<unknown>,
}));
vi.mock("@/lib/api", () => ({
  getApi: () => ({
    listInvoices: (filter: unknown) => {
      backend.calls.push(filter);
      return backend.impl(filter);
    },
  }),
}));

import { GET } from "./route";

const call = (query: string) => GET(new NextRequest(`http://localhost/export/invoices?${query}`));
const failWith = (error: Error) =>
  (backend.impl = async () => {
    throw error;
  });

beforeEach(() => {
  backend.calls.length = 0;
  backend.impl = async () => [];
});

describe("GET /export/invoices", () => {
  it("downloads the invoices of one direction as CSV", async () => {
    const res = await call("format=csv&direction=payable");
    expect(backend.calls).toEqual([{ direction: "payable" }]);
    expect(res.status).toBe(200);
    expect(res.headers.get("content-type")).toBe("text/csv; charset=utf-8");
    expect(res.headers.get("content-disposition")).toMatch(/^attachment; filename="kerdos-invoices-payable-\d{4}-\d{2}-\d{2}\.csv"$/);
    expect(await res.text()).toContain("Number,Direction,Counterparty");
  });

  it("rejects other formats and unknown directions before calling the backend", async () => {
    for (const q of ["format=xlsx", "format=pdf", "", "format=csv&direction=sideways"]) {
      expect((await call(q)).status).toBe(400);
    }
    expect(backend.calls).toEqual([]);
  });

  it("passes a backend error through with its message", async () => {
    failWith(new ApiError(409, "empty", "No company yet."));
    const res = await call("format=csv");
    expect(res.status).toBe(409);
    expect(await res.json()).toEqual({ code: "empty", message: "No company yet." });
  });

  it("reports an unreachable backend as 502", async () => {
    failWith(new ApiError(0, "unreachable", "down"));
    expect((await call("format=csv")).status).toBe(502);
  });
});
