(function () {
  const TONES = ["polite", "firm", "final"];

  function parseAmount(raw) {
    let s = raw.replace(/[^\d.,]/g, "");
    if (/,\d{3}(?!\d)/.test(s)) s = s.replace(/,/g, "");
    else s = s.replace(",", ".");
    const n = Number(s);
    return Number.isFinite(n) ? n : NaN;
  }

  function parseInvoiceCommand(text) {
    const m = /^\s*(?:invoice|bill|charge)\s+(.*)$/i.exec(text || "");
    if (!m) return null;
    const rest = m[1];
    const c = /^([\p{L}][\p{L}'’-]*(?:\s+(?!for\b|at\b|to\b)[\p{L}][\p{L}'’-]*)*)/u.exec(rest);
    if (!c) return null;
    const client = c[1];
    const tail = rest.slice(client.length);

    const h = /(\d+(?:[.,]\d+)?)\s*(?:hours?|hrs?)\s*(?:at|@|x)\s*€?\s*(\d[\d.,]*)/i.exec(tail);
    if (h) {
      const hours = parseAmount(h[1]);
      const rate = parseAmount(h[2]);
      const amount = hours * rate;
      if (!(amount > 0)) return null;
      return { client, amount, description: `${hours} hours at €${rate}` };
    }
    const a = /€?\s*(\d[\d.,]*)/.exec(tail);
    if (!a) return null;
    const amount = parseAmount(a[1]);
    if (!(amount > 0)) return null;
    return { client, amount, description: "Services" };
  }

  function reminderTone(reminderNumber) {
    return TONES[Math.min(Math.max(reminderNumber, 1), TONES.length) - 1];
  }

  function forecast(state, horizonDays = 30) {
    const points = [{ day: 0, balance: state.balance }];
    let balance = state.balance;
    for (let d = 1; d <= horizonDays; d++) {
      const abs = state.day + d;
      for (const e of state.inflows) if (e.day === abs) balance += e.amount;
      for (const e of state.outflows) if (e.day === abs) balance -= e.amount;
      points.push({ day: d, balance });
    }
    return points;
  }

  function shortfall(points) {
    const firstNeg = points.find((p) => p.balance < 0);
    if (!firstNeg) return null;
    const lowest = points.reduce((lo, p) => (p.balance < lo.balance ? p : lo));
    return { firstDay: firstNeg.day, lowestDay: lowest.day, amount: -lowest.balance };
  }

  function advanceDays(state, n) {
    let s = state;
    for (let i = 0; i < n; i++) {
      const day = s.day + 1;
      let balance = s.balance;
      for (const e of s.inflows) if (e.day === day) balance += e.amount;
      for (const e of s.outflows) if (e.day === day) balance -= e.amount;
      s = {
        ...s,
        day,
        balance,
        inflows: s.inflows.filter((e) => e.day !== day),
        outflows: s.outflows.filter((e) => e.day !== day),
        invoices: s.invoices.map((inv) => {
          const age = day - inv.sentDay;
          if (inv.status !== "unpaid" || age <= 0 || age % s.reminderEveryDays !== 0) return inv;
          const tone = reminderTone(inv.reminders.length + 1);
          return { ...inv, reminders: [...inv.reminders, { day, tone }] };
        }),
      };
    }
    return s;
  }

  function remindNow(state, invoiceId) {
    const inv = state.invoices.find((i) => i.id === invoiceId);
    if (!inv || inv.status !== "unpaid" || inv.reminders.some((r) => r.day === state.day)) return state;
    const tone = reminderTone(inv.reminders.length + 1);
    return {
      ...state,
      invoices: state.invoices.map((i) =>
        i.id === invoiceId ? { ...i, reminders: [...i.reminders, { day: state.day, tone }] } : i
      ),
    };
  }

  function applyFinancing(state, amount) {
    if (state.financed) return state;
    const unpaid = state.invoices.filter((i) => i.status === "unpaid");
    if (unpaid.reduce((sum, i) => sum + i.amount, 0) < amount) return state;
    const newestFirst = [...unpaid].sort((a, b) => b.sentDay - a.sentDay);
    const chosen = new Set();
    let covered = 0;
    for (const inv of newestFirst) {
      if (covered >= amount) break;
      chosen.add(inv.id);
      covered += inv.amount;
    }
    return {
      ...state,
      balance: state.balance + amount,
      financed: true,
      invoices: state.invoices.map((i) => (chosen.has(i.id) ? { ...i, status: "financed" } : i)),
    };
  }

  const api = { parseInvoiceCommand, reminderTone, forecast, shortfall, advanceDays, remindNow, applyFinancing };
  if (typeof module !== "undefined") module.exports = api;
  else window.KumbaresLogic = api;
})();
