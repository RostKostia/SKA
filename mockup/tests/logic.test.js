const test = require("node:test");
const assert = require("node:assert/strict");
const L = require("../logic.js");
const DATA = require("../data.js");

const clone = (o) => structuredClone(o);
const invoice = (over) => ({
  id: "t",
  client: "Test",
  amount: 100,
  description: "Services",
  sentDay: 0,
  dueDay: 14,
  status: "unpaid",
  reminders: [],
  ...over,
});
const bare = (over) => ({
  ...clone(DATA),
  invoices: [],
  inflows: [],
  outflows: [],
  ...over,
});

test("parseInvoiceCommand reads client and amount from a short command", () => {
  const r = L.parseInvoiceCommand("Invoice Kostas €120");
  assert.equal(r.client, "Kostas");
  assert.equal(r.amount, 120);
  assert.equal(r.description, "Services");
});

test("parseInvoiceCommand multiplies hours by rate", () => {
  const r = L.parseInvoiceCommand("Invoice Kostas, 3 hours at €40");
  assert.equal(r.client, "Kostas");
  assert.equal(r.amount, 120);
  assert.equal(r.description, "3 hours at €40");
});

test("parseInvoiceCommand reads decimal and thousands amounts", () => {
  assert.equal(L.parseInvoiceCommand("Invoice Kostas €40.50").amount, 40.5);
  assert.equal(L.parseInvoiceCommand("Invoice Kostas 40,50").amount, 40.5);
  assert.equal(L.parseInvoiceCommand("Invoice Kostas €1,200").amount, 1200);
});

test("parseInvoiceCommand keeps a two-word client name", () => {
  assert.equal(L.parseInvoiceCommand("Invoice Maria Savva €300").client, "Maria Savva");
});

test("parseInvoiceCommand returns null when client or amount is missing", () => {
  assert.equal(L.parseInvoiceCommand("Invoice Kostas"), null);
  assert.equal(L.parseInvoiceCommand("Invoice €120"), null);
  assert.equal(L.parseInvoiceCommand(""), null);
  assert.equal(L.parseInvoiceCommand("   "), null);
});

test("reminderTone escalates and stops at final", () => {
  assert.deepEqual([1, 2, 3, 7].map(L.reminderTone), ["polite", "firm", "final", "final"]);
});

test("forecast and shortfall find the dip in the sample data", () => {
  const points = L.forecast(DATA);
  assert.equal(points.length, 31);
  assert.deepEqual(points[0], { day: 0, balance: 1000 });
  assert.deepEqual(L.shortfall(points), { firstDay: 4, lowestDay: 12, amount: 800 });
});

test("shortfall is null when the balance never goes below zero", () => {
  assert.equal(L.shortfall(L.forecast({ ...clone(DATA), balance: 5000 })), null);
});

test("advanceDays applies passed events to the balance and moves the forecast", () => {
  const s = L.advanceDays(DATA, 3);
  assert.equal(s.day, 3);
  assert.equal(s.balance, 1300);
  assert.deepEqual(L.shortfall(L.forecast(s)), { firstDay: 1, lowestDay: 9, amount: 800 });
});

test("advanceDays sends one reminder per interval", () => {
  const s = L.advanceDays(bare({ invoices: [invoice({ sentDay: 0 })] }), 3);
  assert.deepEqual(s.invoices[0].reminders, [{ day: 3, tone: "polite" }]);
});

test("advanceDays escalates polite, firm, final, final", () => {
  let s = bare({ invoices: [invoice({ sentDay: 0 })] });
  for (let i = 0; i < 4; i++) s = L.advanceDays(s, 3);
  assert.deepEqual(
    s.invoices[0].reminders.map((r) => r.tone),
    ["polite", "firm", "final", "final"]
  );
});

test("advanceDays by zero changes nothing and never duplicates reminders", () => {
  const s0 = bare({ invoices: [invoice({ sentDay: 0 })] });
  assert.deepEqual(L.advanceDays(s0, 0), s0);
  const s = L.advanceDays(L.advanceDays(s0, 3), 0);
  assert.equal(s.invoices[0].reminders.length, 1);
});

test("advanceDays does not remind paid or financed invoices", () => {
  const s = L.advanceDays(
    bare({ invoices: [invoice({ id: "a", status: "paid" }), invoice({ id: "b", status: "financed" })] }),
    3
  );
  assert.deepEqual(s.invoices.map((i) => i.reminders.length), [0, 0]);
});

test("advanceDays does not change the state it was given", () => {
  const before = clone(DATA);
  L.advanceDays(DATA, 3);
  assert.deepEqual(DATA, before);
});

test("remindNow adds the next-tone reminder today, once", () => {
  const s0 = bare({ day: 5, invoices: [invoice({ reminders: [{ day: 3, tone: "polite" }] })] });
  const s1 = L.remindNow(s0, "t");
  assert.deepEqual(s1.invoices[0].reminders.at(-1), { day: 5, tone: "firm" });
  assert.deepEqual(L.remindNow(s1, "t"), s1);
});

test("remindNow ignores paid invoices and unknown ids", () => {
  const s0 = bare({ invoices: [invoice({ status: "paid" })] });
  assert.deepEqual(L.remindNow(s0, "t"), s0);
  assert.deepEqual(L.remindNow(s0, "missing"), s0);
});

test("applyFinancing credits the balance and finances newest invoices first", () => {
  const s = bare({
    balance: 0,
    invoices: [
      invoice({ id: "old", amount: 450, sentDay: -10 }),
      invoice({ id: "mid", amount: 380, sentDay: -5 }),
      invoice({ id: "new", amount: 120, sentDay: 0 }),
    ],
  });
  const r = L.applyFinancing(s, 800);
  assert.equal(r.balance, 800);
  assert.equal(r.financed, true);
  assert.deepEqual(r.invoices.map((i) => i.status), ["financed", "financed", "financed"]);
  const small = L.applyFinancing(s, 300);
  assert.deepEqual(small.invoices.map((i) => i.status), ["unpaid", "financed", "financed"]);
});

test("applyFinancing is a no-op the second time and when invoices cannot cover it", () => {
  const s = bare({ balance: 0, invoices: [invoice({ amount: 900 })] });
  const once = L.applyFinancing(s, 800);
  assert.deepEqual(L.applyFinancing(once, 800), once);
  assert.deepEqual(L.applyFinancing(s, 5000), s);
});

test("financing the sample shortfall removes the dip", () => {
  const s = L.applyFinancing(L.advanceDays(DATA, 3), 800);
  assert.equal(L.shortfall(L.forecast(s)), null);
});
