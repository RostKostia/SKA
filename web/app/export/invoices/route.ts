import type { NextRequest } from "next/server";
import { getApi } from "@/lib/api";
import { ApiError } from "@/lib/api/client";
import { invoicesToCsv } from "@/lib/csv";

const error = (status: number, code: string, message: string) => Response.json({ code, message }, { status });

// CSV download of the invoice list. The backend has no export endpoint yet, so the file is
// formatted here from the same list the invoices page shows.
export async function GET(request: NextRequest) {
  const params = request.nextUrl.searchParams;
  const direction = params.get("direction");
  if (params.get("format") !== "csv") return error(400, "bad_request", "Only CSV export is available.");
  if (direction !== null && direction !== "receivable" && direction !== "payable") {
    return error(400, "bad_request", "direction must be receivable or payable.");
  }

  try {
    const invoices = await getApi().listInvoices(direction ? { direction } : {});
    const day = new Date().toISOString().slice(0, 10);
    return new Response(invoicesToCsv(invoices), {
      headers: {
        "content-type": "text/csv; charset=utf-8",
        "content-disposition": `attachment; filename="kerdos-invoices-${direction ?? "all"}-${day}.csv"`,
      },
    });
  } catch (e) {
    if (!(e instanceof ApiError)) throw e;
    return error(e.status === 0 ? 502 : e.status, e.code, e.message);
  }
}
