import type { Invoice } from "./api/types";

const HEADER = ["Number", "Direction", "Counterparty", "Amount", "Currency", "Issue date", "Due date", "Status"];

// A leading = + - @ makes spreadsheets run the cell as a formula, so text cells get a quote prefix.
function text(value: string): string {
  return cell(/^[=+\-@\t\r]/.test(value) ? `'${value}` : value);
}

function cell(value: string): string {
  return /[",\r\n]/.test(value) ? `"${value.replace(/"/g, '""')}"` : value;
}

// The BOM makes Excel read the file as UTF-8, so Greek names survive.
export function invoicesToCsv(invoices: Invoice[]): string {
  const rows = invoices.map((i) =>
    [
      text(i.number),
      text(i.direction),
      text(i.counterparty_name),
      (i.amount_cents / 100).toFixed(2),
      text(i.currency),
      i.issue_date,
      i.due_date,
      text(i.display_status),
    ].join(","),
  );
  return `﻿${[HEADER.join(","), ...rows].join("\r\n")}\r\n`;
}
