(function () {
  const L = window.KumbaresLogic;
  const TODAY = new Date(2026, 10, 27);
  const SKIP_DAYS = 3;
  const TONE_LABEL = { polite: "Polite", firm: "Firm", final: "Final notice" };

  let state = structuredClone(KUMBARES_DATA);
  let view = "chat";
  let clientInvoiceId = null;
  let chat = [
    {
      from: "kumbares",
      text: "Good morning. Tell me who to invoice and I'll send it through Jinius with a payment link.",
      chips: ["Invoice Kostas €120", "Invoice Kostas, 3 hours at €40"],
    },
  ];

  const $ = (sel) => document.querySelector(sel);
  const esc = (s) => String(s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
  const eur = (n) => (n < 0 ? "−" : "") + "€" + Math.abs(Math.round(n * 100) / 100).toLocaleString("en-GB", { maximumFractionDigits: 2 });
  const dayDate = (d) => {
    const x = new Date(TODAY);
    x.setDate(x.getDate() + d);
    return x;
  };
  const shortDate = (d) => dayDate(d).toLocaleDateString("en-GB", { day: "numeric", month: "short" });
  const longDate = (d) => dayDate(d).toLocaleDateString("en-GB", { day: "numeric", month: "short", year: "numeric" });
  const invNo = (inv) => "KB-" + (1040 + Number(inv.id.split("-")[1]));
  const plural = (n, w) => `${n} ${w}${n === 1 ? "" : "s"}`;

  function statusOf(inv) {
    if (inv.status === "paid") return { cls: "paid", icon: "✓", label: "Paid" };
    if (inv.status === "financed") return { cls: "financed", icon: "⇄", label: "Financed" };
    if (state.day > inv.dueDay) return { cls: "overdue", icon: "!", label: "Overdue" };
    return { cls: "unpaid", icon: "○", label: "Unpaid" };
  }
  const statusHtml = (inv) => {
    const s = statusOf(inv);
    return `<span class="status ${s.cls}"><span class="ic" aria-hidden="true">${s.icon}</span>${s.label}</span>`;
  };

  function toneMessage(inv, tone, day = state.day) {
    const age = day - inv.sentDay;
    const who = inv.client;
    if (tone === "polite") return `Hi ${who}, a friendly reminder that invoice ${invNo(inv)} for ${eur(inv.amount)} is still open. You can pay in one tap with the link below.`;
    if (tone === "firm") return `Hi ${who}, invoice ${invNo(inv)} (${eur(inv.amount)}) is now ${plural(age, "day")} old and still unpaid. Please pay it today using the link below.`;
    return `Final notice: invoice ${invNo(inv)} (${eur(inv.amount)}) is ${plural(age, "day")} old. Please pay today or contact us to agree a plan.`;
  }

  /* ---------- derived data ---------- */
  const unpaid = () => state.invoices.filter((i) => i.status === "unpaid");
  const currentShortfall = () => L.shortfall(L.forecast(state));
  function outflowOn(rel) {
    return state.outflows.find((e) => e.day === state.day + rel);
  }
  function owedByClient() {
    const map = new Map();
    for (const inv of unpaid()) {
      const row = map.get(inv.client) || { client: inv.client, total: 0, oldest: 0, invoices: [] };
      row.total += inv.amount;
      row.oldest = Math.max(row.oldest, state.day - inv.sentDay);
      row.invoices.push(inv);
      map.set(inv.client, row);
    }
    return [...map.values()].sort((a, b) => b.total - a.total);
  }
  function lastTone(invoices) {
    const all = invoices.flatMap((i) => i.reminders).sort((a, b) => a.day - b.day);
    return all.length ? TONE_LABEL[all[all.length - 1].tone] : "None yet";
  }

  /* ---------- chart ---------- */
  function renderForecastChart(points, info, container, opts = {}) {
    const W = opts.width || 340;
    const H = opts.height || 190;
    const m = { l: 46, r: 12, t: 14, b: 28 };
    const id = opts.id || "c";
    const bal = points.map((p) => p.balance);
    const pad = (Math.max(...bal, 0) - Math.min(...bal, 0)) * 0.14 || 100;
    const hi = Math.max(...bal, 0) + pad;
    const lo = Math.min(...bal, 0) - pad;
    const x = (d) => m.l + (d / (points.length - 1)) * (W - m.l - m.r);
    const y = (v) => m.t + ((hi - v) / (hi - lo)) * (H - m.t - m.b);
    const y0 = y(0);

    let line = `M${x(0)},${y(points[0].balance)}`;
    for (let i = 1; i < points.length; i++) line += ` H${x(i)} V${y(points[i].balance)}`;
    const area = `${line} V${y0} H${x(0)} Z`;

    const desc = info
      ? `Projected balance over the next ${points.length - 1} days. It starts at ${eur(points[0].balance)}, first goes below zero on ${shortDate(state.day + info.firstDay)}, and reaches its lowest point of ${eur(-info.amount)} on ${shortDate(state.day + info.lowestDay)}.`
      : `Projected balance over the next ${points.length - 1} days. It starts at ${eur(points[0].balance)} and stays above zero.`;

    let marks = "";
    if (opts.events) {
      opts.events.forEach((e, i) => {
        const p = points[e.rel];
        marks += `<circle cx="${x(e.rel)}" cy="${y(p.balance)}" r="11" fill="${e.kind === "in" ? "#0f766e" : "#0f3a40"}" stroke="#fff" stroke-width="2"/><text class="ev" x="${x(e.rel)}" y="${y(p.balance) + 4}" text-anchor="middle">${i + 1}</text>`;
      });
    }
    let lowDot = "";
    let lowLabel = "";
    if (info) {
      const lx = x(info.lowestDay);
      const ly = y(-info.amount);
      const anchor = lx > W * 0.55 ? "end" : "start";
      const tx = anchor === "end" ? lx - 8 : lx + 8;
      lowDot = `<circle cx="${lx}" cy="${ly}" r="5" fill="#b3401b"/>`;
      lowLabel = `<text class="lbl-bad" x="${tx}" y="${ly - 8}" text-anchor="${anchor}">Day ${info.lowestDay}: ${eur(-info.amount)}</text>`;
    }
    const ticks = [0, 10, 20, 30]
      .filter((d) => d < points.length)
      .map((d) => `<text x="${x(d)}" y="${H - 6}" text-anchor="${d === 0 ? "start" : d === points.length - 1 ? "end" : "middle"}">${d === 0 ? "Today" : shortDate(state.day + d)}</text>`)
      .join("");
    const yTicks = [...new Set([Math.max(...bal) > 0 ? Math.max(...bal) : 0, 0])]
      .map((v) => `<text x="${m.l - 6}" y="${y(v) + 4}" text-anchor="end">${eur(Math.round(v / 100) * 100)}</text>`)
      .join("");

    container.innerHTML = `<svg class="chart" viewBox="0 0 ${W} ${H}" role="img" aria-label="${esc(desc)}">
      <defs>
        <pattern id="hatch-${id}" width="6" height="6" patternUnits="userSpaceOnUse" patternTransform="rotate(45)"><rect width="6" height="6" fill="#fbede7"/><line x1="0" y1="0" x2="0" y2="6" stroke="#b3401b" stroke-width="2"/></pattern>
        <clipPath id="above-${id}"><rect x="0" y="0" width="${W}" height="${y0}"/></clipPath>
        <clipPath id="below-${id}"><rect x="0" y="${y0}" width="${W}" height="${H - y0}"/></clipPath>
      </defs>
      <path d="${area}" fill="#e1f0ee" clip-path="url(#above-${id})"/>
      <path d="${area}" fill="url(#hatch-${id})" clip-path="url(#below-${id})"/>
      <line x1="${m.l}" x2="${W - m.r}" y1="${y0}" y2="${y0}" stroke="#3b5f64" stroke-width="1.5"/>
      <path d="${line}" fill="none" stroke="#0f3a40" stroke-width="2.5" stroke-linejoin="round"/>
      ${yTicks}${ticks}${lowDot}${marks}${lowLabel}
    </svg>`;
  }

  /* ---------- views ---------- */
  let lastAnnounced = null;
  function renderChat() {
    const log = $("#chat-log");
    log.innerHTML = chat.map(msgHtml).join("");
    log.scrollTop = log.scrollHeight;
    const last = chat[chat.length - 1];
    if (last !== lastAnnounced) {
      lastAnnounced = last;
      const c = last.card;
      const extra = c && c.type === "warning" ? ` In ${plural(c.lowestDay, "day")} you'll be ${eur(c.amount)} negative.` : c && c.type === "invoice" ? " Invoice sent." : "";
      $("#announce").textContent = `${last.from === "me" ? "You: " : ""}${last.text}${extra}`;
    }
  }

  function cardHtml(c) {
    if (c.type === "invoice") {
      const inv = state.invoices.find((i) => i.id === c.id);
      return `<div class="paper"><div class="row"><h3>${invNo(inv)} · ${esc(inv.client)}</h3><span class="tag">Sample</span></div>
        <div class="amount">${eur(inv.amount)}</div><p>${esc(inv.description)}</p>
        <ul><li>✓ Sent via Jinius with a payment link</li><li>✓ Copy sent by email</li><li>Due ${longDate(inv.dueDay)}</li></ul></div>`;
    }
    if (c.type === "reminder") {
      const inv = state.invoices.find((i) => i.id === c.id);
      return `<div class="paper"><div class="row"><h3>Reminder to ${esc(inv.client)}</h3><span class="tag">${TONE_LABEL[c.tone]} · email</span></div><p class="quote">${esc(c.message)}</p></div>`;
    }
    if (c.type === "warning") {
      const where = c.firstOut ? ` The first dip comes on ${shortDate(c.firstDate)}, when ${esc(c.firstOut.label.toLowerCase())} (${eur(c.firstOut.amount)}) goes out.` : "";
      const actions = state.financed
        ? `<p class="done">Done. The advance is in your account.</p>`
        : `<div class="acts"><button type="button" class="btn primary" data-act="finance" data-amount="${c.amount}">Get ${eur(c.amount)} now</button><button type="button" class="btn" data-act="remind-all">Remind all clients first</button></div>`;
      return `<div class="alertbox"><strong>In ${plural(c.lowestDay, "day")} you'll be ${eur(c.amount)} negative.</strong>
        <p>That's based on your balance and the bills coming up.${where} Your unpaid invoices total ${eur(c.unpaidTotal)}, so I can advance ${eur(c.amount)} against them.</p>${actions}</div>`;
    }
    if (c.type === "financed") {
      return `<div class="okbox"><h3>${eur(c.amount)} advanced against your unpaid invoices</h3><p class="muted">Simulated. A real offer would show the fee and repayment terms before you accept.</p></div>`;
    }
    return "";
  }

  function msgHtml(m) {
    if (m.from === "system") return `<div class="sys">${esc(m.text)}</div>`;
    if (m.from === "me") return `<div class="msg me"><p>${esc(m.text)}</p></div>`;
    const chips = (m.chips || []).map((t) => `<button type="button" class="chip" data-act="fill" data-text="${esc(t)}">${esc(t)}</button>`).join("");
    return `<div class="msg"><span class="avatar" aria-hidden="true">κ</span><div class="body"><p>${esc(m.text)}</p>${m.card ? cardHtml(m.card) : ""}${chips ? `<div class="chips">${chips}</div>` : ""}</div></div>`;
  }

  function renderInvoices() {
    const rows = state.invoices
      .slice()
      .sort((a, b) => b.sentDay - a.sentDay)
      .map((inv) => {
        const last = inv.reminders.at(-1);
        const act = inv.status === "unpaid" ? `<button type="button" class="btn small" data-act="remind" data-id="${inv.id}" aria-label="Remind ${esc(inv.client)} now">Remind now</button>` : "";
        return `<tr><td>${invNo(inv)}</td><td class="wrap"><strong>${esc(inv.client)}</strong><br><span class="muted">${esc(inv.description)}</span></td>
          <td class="r num">${eur(inv.amount)}</td><td>${shortDate(inv.sentDay)}</td><td>${shortDate(inv.dueDay)}</td><td>${statusHtml(inv)}</td>
          <td>${inv.reminders.length ? `${inv.reminders.length} sent, last ${TONE_LABEL[last.tone].toLowerCase()}` : "None"}</td><td>${act}</td></tr>`;
      })
      .join("");
    $("#view-invoices").innerHTML = `<div class="page-head"><h1>Invoices</h1><p class="muted">Every invoice sent through Jinius, newest first.</p></div>
      <table><thead><tr><th scope="col">No.</th><th scope="col">Client</th><th scope="col" class="r">Amount</th><th scope="col">Sent</th><th scope="col">Due</th><th scope="col">Status</th><th scope="col">Reminders</th><th scope="col"><span class="sr">Action</span></th></tr></thead><tbody>${rows}</tbody></table>`;
  }

  function renderDebtors() {
    const list = owedByClient();
    const total = list.reduce((s, r) => s + r.total, 0);
    const cards = list
      .map(
        (r) => `<div class="debtor"><div><h3>${esc(r.client)}</h3></div><div class="big">${eur(r.total)}</div>
        <button type="button" class="btn small" data-act="remind" data-id="${r.invoices[0].id}" aria-label="Remind ${esc(r.client)} now">Remind now</button>
        <div class="meta">Oldest invoice ${plural(r.oldest, "day")} old · Reminders sent: ${r.invoices.reduce((s, i) => s + i.reminders.length, 0)} · Last tone: ${lastTone(r.invoices)}</div></div>`
      )
      .join("");
    $("#view-debtors").innerHTML = `<div class="page-head"><h1>${list.length ? `${eur(total)} owed by ${plural(list.length, "client")}` : "Nobody owes you anything"}</h1><p class="muted">Sorted by what you're owed.</p></div>
      <div class="debtor-list">${cards || '<p>All invoices are paid or financed. New invoices will show up here.</p>'}</div>`;
  }

  function renderReminders() {
    const sample = { id: "inv-3", client: "Kostas", amount: 120, sentDay: state.day - 9 };
    const steps = [
      ["polite", "Reminder 1", "A friendly nudge with the payment link."],
      ["firm", "Reminder 2", "States how many days the invoice is open and asks for payment today."],
      ["final", "Reminder 3 and after", "A final notice. Stays at this level and asks for a call or a plan."],
    ]
      .map(
        ([tone, name, what], i) => `<li><span class="n" aria-hidden="true">${i + 1}</span><div><h3>${name}: ${TONE_LABEL[tone].toLowerCase()}</h3><p class="muted">${what}</p><p class="sample">${esc(toneMessage(sample, tone))}</p></div></li>`
      )
      .join("");
    $("#view-reminders").innerHTML = `<div class="page-head"><h1>Reminders</h1><p class="muted">Kumbares chases unpaid invoices for you, so you don't have to. Reminders go by email in this version.</p>
      <div class="interval"><span class="muted">Remind every</span><strong class="num">${state.reminderEveryDays} days</strong></div></div><ol class="steps">${steps}</ol>`;
  }

  function eventsList() {
    const evs = [
      ...state.inflows.map((e) => ({ ...e, kind: "in" })),
      ...state.outflows.map((e) => ({ ...e, kind: "out" })),
    ]
      .filter((e) => e.day > state.day && e.day <= state.day + 30)
      .sort((a, b) => a.day - b.day)
      .map((e) => ({ ...e, rel: e.day - state.day }));
    return evs;
  }

  function renderForecastView() {
    const points = L.forecast(state);
    const info = L.shortfall(points);
    const evs = eventsList();
    const items = evs
      .map((e, i) => `<li class="${e.kind}"><span class="dot" aria-hidden="true">${i + 1}</span><span><strong>${shortDate(e.day)}</strong> · ${esc(e.label)} · <span class="num">${e.kind === "in" ? "+" : "−"}${eur(e.amount)}</span></span></li>`)
      .join("");
    $("#view-forecast").innerHTML = `<div class="forecast-wrap"><div class="page-head"><h1>${info ? `In ${plural(info.lowestDay, "day")} you'll be ${eur(info.amount)} negative` : "No shortfall in the next 30 days"}</h1>
      <p class="muted">Projection from your account balance (${eur(state.balance)}, sample) and known payments in and out. Unpaid invoices aren't counted until they're paid, because these clients usually pay late.</p></div>
      <div id="chart-big"></div><ol class="events" aria-label="Payments in and out">${items}</ol></div>`;
    renderForecastChart(points, info, $("#chart-big"), { width: 800, height: 320, id: "big", events: evs });
  }

  function renderSide() {
    const points = L.forecast(state);
    const info = L.shortfall(points);
    const owed = owedByClient();
    const remindBtns = owed
      .map((r) => `<button type="button" class="btn small" data-act="remind" data-id="${r.invoices[0].id}">Remind ${esc(r.client)} now</button>`)
      .join("");
    const finance = info && !state.financed ? `<button type="button" class="btn primary" data-act="finance" data-amount="${info.amount}">Get ${eur(info.amount)} now</button>` : "";
    $("#side-panel").innerHTML = `
      <section><h2>Cash outlook</h2>
        <p class="headline ${info ? "bad" : ""}">${info ? `In ${plural(info.lowestDay, "day")} you'll be ${eur(info.amount)} negative.` : "No shortfall in the next 30 days."}</p>
        <div id="chart-side"></div></section>
      <section><h2>Who owes you</h2>${owed.length ? `<ul class="owe">${owed.map((r) => `<li><span>${esc(r.client)}</span><span class="num">${eur(r.total)}</span></li>`).join("")}</ul>` : '<p class="muted">Nobody. All invoices are paid or financed.</p>'}</section>
      <section><h2>Action plan</h2><div class="plan">${finance}${remindBtns}${!finance && !remindBtns ? '<p class="muted">Nothing to do right now.</p>' : ""}</div>
      ${finance ? `<p class="muted">Advance against your unpaid invoices. Simulated in this mockup.</p>` : ""}</section>`;
    renderForecastChart(points, info, $("#chart-side"), { id: "side" });
  }

  function renderClient() {
    const inv = state.invoices.find((i) => i.id === clientInvoiceId) || state.invoices.filter((i) => i.status === "unpaid").at(-1) || state.invoices.at(-1);
    if (!inv) {
      $("#view-client").innerHTML = `<p class="client-note">No invoice sent yet. Create one in Chat and it will show up here.</p>`;
      return;
    }
    const last = inv.reminders.at(-1);
    const banner =
      inv.status === "unpaid" && last
        ? `<div class="banner ${last.tone}"><p><strong>${TONE_LABEL[last.tone]} reminder</strong></p><p>${esc(toneMessage(inv, last.tone))}</p></div>`
        : "";
    const done = inv.status === "paid";
    $("#view-client").innerHTML = `<div class="phone">
        <div class="from"><span class="avatar" aria-hidden="true">${esc(state.business[0])}</span>${esc(state.business)}</div>
        ${banner}
        <div><p class="muted">Invoice ${invNo(inv)}</p><p class="amount">${eur(inv.amount)}</p></div>
        <dl><dt>To</dt><dd>${esc(inv.client)}</dd><dt>For</dt><dd>${esc(inv.description)}</dd><dt>Due</dt><dd>${longDate(inv.dueDay)}</dd><dt>Status</dt><dd>${statusHtml(inv)}</dd></dl>
        ${done ? `<div class="okbox"><h3>Paid, thank you</h3><p class="muted">Simulated payment. ${esc(inv.client)} would get a receipt by email.</p></div>` : `<button type="button" class="btn primary" data-act="pay" data-id="${inv.id}">Pay ${eur(inv.amount)} now</button>`}
      </div>
      <p class="client-note">This is what ${esc(inv.client)} sees on their phone. Sample data, no real payment.</p>`;
  }

  function render() {
    $("#app").dataset.view = view;
    document.querySelectorAll(".nav [data-view]").forEach((b) => (b.getAttribute("data-view") === view ? b.setAttribute("aria-current", "page") : b.removeAttribute("aria-current")));
    document.querySelectorAll(".main > section").forEach((s) => (s.hidden = s.id !== `view-${view}`));
    $("#biz").textContent = state.business;
    $("#today").textContent = state.day === 0 ? `Today, ${longDate(0)}` : `${longDate(state.day)} · day ${state.day}`;
    $("#btn-client-view").textContent = view === "client" ? "Back to owner app" : "Client view";
    renderChat();
    renderInvoices();
    renderDebtors();
    renderReminders();
    renderForecastView();
    renderClient();
    renderSide();
  }

  function setState(next) {
    state = next;
    render();
  }

  function showView(name) {
    view = name;
    render();
  }

  /* ---------- actions ---------- */
  function handleChatSubmit(text) {
    const t = text.trim();
    if (!t) return;
    chat.push({ from: "me", text: t });
    const parsed = L.parseInvoiceCommand(t);
    if (!parsed) {
      chat.push({ from: "kumbares", text: "I need a client and an amount to make an invoice. Try: Invoice Kostas, 3 hours at €40", chips: ["Invoice Kostas €120"] });
      return render();
    }
    const n = Math.max(...state.invoices.map((i) => Number(i.id.split("-")[1]))) + 1;
    const inv = { id: `inv-${n}`, client: parsed.client, amount: parsed.amount, description: parsed.description, sentDay: state.day, dueDay: state.day + 14, status: "unpaid", reminders: [] };
    clientInvoiceId = inv.id;
    chat.push({
      from: "kumbares",
      text: `Done. I sent it through Jinius with a payment link and emailed a copy. I'll remind ${inv.client} every ${state.reminderEveryDays} days until it's paid.`,
      card: { type: "invoice", id: inv.id },
    });
    setState({ ...state, invoices: [...state.invoices, inv] });
  }

  function handleSkipTime() {
    const before = state;
    const after = L.advanceDays(state, SKIP_DAYS);
    chat.push({ from: "system", text: `${SKIP_DAYS} days later · ${longDate(after.day)}` });
    for (const e of before.inflows.filter((e) => e.day <= after.day)) {
      chat.push({ from: "kumbares", text: `${e.label.replace(/ \(sample\)$/, "")} of ${eur(e.amount)} arrived.` });
    }
    for (const inv of after.invoices) {
      for (const r of inv.reminders.filter((r) => r.day > before.day)) {
        chat.push({ from: "kumbares", text: `I reminded ${inv.client}.`, card: { type: "reminder", id: inv.id, tone: r.tone, message: toneMessage(inv, r.tone, r.day) } });
      }
    }
    chat = chat.filter((m) => !(m.card && m.card.type === "warning"));
    const info = L.shortfall(L.forecast(after));
    if (info && !after.financed) {
      const firstOut = after.outflows.find((e) => e.day === after.day + info.firstDay);
      chat.push({ from: "kumbares", text: "Heads up.", card: { type: "warning", lowestDay: info.lowestDay, amount: info.amount, firstOut, firstDate: after.day + info.firstDay, unpaidTotal: after.invoices.filter((i) => i.status === "unpaid").reduce((s, i) => s + i.amount, 0) } });
    }
    setState(after);
  }

  function handleRemind(id) {
    const next = L.remindNow(state, id);
    const inv = state.invoices.find((i) => i.id === id);
    if (next === state) {
      chat.push({ from: "kumbares", text: `I already reminded ${inv.client} today.` });
      return render();
    }
    const r = next.invoices.find((i) => i.id === id).reminders.at(-1);
    chat.push({ from: "kumbares", text: `I reminded ${inv.client}.`, card: { type: "reminder", id, tone: r.tone, message: toneMessage(inv, r.tone) } });
    setState(next);
  }

  function handleFinancing(amount) {
    const next = L.applyFinancing(state, amount);
    if (next === state) return;
    chat.push({ from: "kumbares", text: "Done. It's in your account.", card: { type: "financed", amount } });
    setState(next);
  }

  function handleRemindAll() {
    let next = state;
    const ids = unpaid().map((i) => i.id);
    for (const id of ids) {
      const after = L.remindNow(next, id);
      if (after !== next) {
        const inv = after.invoices.find((i) => i.id === id);
        const r = inv.reminders.at(-1);
        chat.push({ from: "kumbares", text: `I reminded ${inv.client}.`, card: { type: "reminder", id, tone: r.tone, message: toneMessage(inv, r.tone) } });
      }
      next = after;
    }
    if (next === state) chat.push({ from: "kumbares", text: "Everyone has already been reminded today." });
    setState(next);
  }

  function handlePay(id) {
    setState({ ...state, invoices: state.invoices.map((i) => (i.id === id ? { ...i, status: "paid" } : i)) });
  }

  /* ---------- wiring ---------- */
  document.addEventListener("click", (e) => {
    const el = e.target.closest("[data-act], [data-view]");
    if (!el) return;
    if (el.dataset.view) return showView(el.dataset.view);
    const a = el.dataset.act;
    if (a === "fill") {
      const input = $("#chat-input");
      input.value = el.dataset.text;
      input.focus();
    } else if (a === "finance") handleFinancing(Number(el.dataset.amount));
    else if (a === "remind") handleRemind(el.dataset.id);
    else if (a === "remind-all") handleRemindAll();
    else if (a === "pay") handlePay(el.dataset.id);
  });
  $("#chat-form").addEventListener("submit", (e) => {
    e.preventDefault();
    const input = $("#chat-input");
    handleChatSubmit(input.value);
    input.value = "";
  });
  $("#btn-skip-time").addEventListener("click", handleSkipTime);
  $("#btn-client-view").addEventListener("click", () => showView(view === "client" ? "chat" : "client"));

  render();
})();
