// Kerdos mockup, running on the Kerdos API (same origin: http://localhost:8080/mockup/).
// The backend owns every number and rule; this file only fetches, renders and sends actions.
(function () {
  "use strict";

  const TONE_LABEL = { polite: "Polite", firm: "Firm", final: "Final notice" };
  const STATUS = {
    paid: ["paid", "✓", "Paid"],
    financed: ["financed", "⇄", "Financed"],
    overdue: ["overdue", "!", "Overdue"],
    unpaid: ["unpaid", "○", "Unpaid"],
    partially_paid: ["unpaid", "◐", "Part paid"],
    draft: ["unpaid", "✎", "Draft"],
    cancelled: ["unpaid", "✕", "Cancelled"],
  };

  // ---------- helpers ----------
  const $ = (sel) => document.querySelector(sel);
  const esc = (s) => String(s == null ? "" : s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
  const eur = (cents) => {
    const v = Math.abs(cents) / 100;
    return (cents < 0 ? "−" : "") + "€" + v.toLocaleString("en-GB", { minimumFractionDigits: v % 1 ? 2 : 0, maximumFractionDigits: 2 });
  };
  const toDate = (s) => new Date(String(s).slice(0, 10) + "T00:00:00");
  const shortDate = (s) => toDate(s).toLocaleDateString("en-GB", { day: "numeric", month: "short" });
  const longDate = (s) => toDate(s).toLocaleDateString("en-GB", { day: "numeric", month: "short", year: "numeric" });
  const plural = (n, w) => `${n} ${w}${n === 1 ? "" : "s"}`;
  const statusHtml = (key) => {
    const [cls, icon, label] = STATUS[key] || STATUS.unpaid;
    return `<span class="status ${cls}"><span class="ic" aria-hidden="true">${icon}</span>${label}</span>`;
  };

  async function api(method, path, body) {
    const opts = { method, headers: {} };
    if (body !== undefined) {
      opts.headers["Content-Type"] = "application/json; charset=utf-8";
      opts.body = JSON.stringify(body);
    }
    const res = await fetch(path, opts);
    let data = null;
    try {
      data = await res.json();
    } catch (_) {
      /* empty body */
    }
    if (!res.ok) {
      const err = new Error((data && data.error && data.error.message) || `${res.status} ${res.statusText}`);
      err.status = res.status;
      err.code = data && data.error && data.error.code;
      throw err;
    }
    return data;
  }

  // ---------- state (all of it from the API) ----------
  const s = { dash: null, chat: [], invoices: [], messages: [], settings: null, offers: {}, pay: null, payError: null };
  let view = "chat";
  let invTab = "receivable";
  let clientToken = null;
  let busy = false;
  let fatal = null;
  let noticeTimer = null;
  let lastAnnounced = null;
  let auth = null; // GET /api/auth/status: mode "dev" (no sign-in) or "accounts"
  let authNote = ""; // a sign-in problem to show on the sign-in screen

  const cardOf = (m) => (m && m.card && typeof m.card === "object" ? m.card : null);
  const problem = (err) => {
    const e = String(err || "");
    if (/535|not accepted|login as/i.test(e)) return "the mail server refused the login (check the app password in SMTP_PASSWORD)";
    if (/connect to|no such host|refused/i.test(e)) return "the mail server couldn't be reached";
    if (/timeout|deadline/i.test(e)) return "the mail server didn't answer in time";
    return e.replace(/^mail: /, "").replace(/^not sent: /, "").split("\n")[0].slice(0, 120);
  };
  // One line saying what really happened to an email, with Retry when it failed.
  function emailLine(msg, what) {
    if (!msg) return "";
    if (msg.status === "sent") return `<li>✓ ${esc(what)} emailed to ${esc(msg.recipient)}</li>`;
    if (msg.status === "failed")
      return `<li class="bad-line">✗ ${esc(what)} to ${esc(msg.recipient)} failed: ${esc(problem(msg.error))} <button type="button" class="btn small" data-act="retry-email" data-id="${esc(msg.id)}">Retry</button></li>`;
    if (msg.status === "cancelled") return `<li>${esc(what)} not sent: ${esc(problem(msg.error))}</li>`;
    return `<li>${esc(what)} to ${esc(msg.recipient)} is on its way</li>`;
  }
  const messageById = (id) => s.messages.find((m) => m.id === id);

  async function load() {
    const [dash, chat, inv, msgs, settings] = await Promise.all([
      api("GET", "/api/dashboard"),
      api("GET", "/api/chat?limit=200"),
      api("GET", "/api/invoices"),
      api("GET", "/api/messages?limit=200"),
      api("GET", "/api/reminders/settings"),
    ]);
    s.dash = dash;
    s.chat = chat.messages || [];
    s.invoices = inv.invoices || [];
    s.messages = msgs.messages || [];
    s.settings = settings;
    const offerIds = [...new Set(s.chat.map(cardOf).filter((c) => c && c.type === "offer").map((c) => c.offer_id))];
    await Promise.all(
      offerIds.map(async (id) => {
        try {
          s.offers[id] = await api("GET", "/api/financing/offers/" + id);
        } catch (_) {
          delete s.offers[id];
        }
      })
    );
    if (view === "client") await loadClient();
  }

  async function refresh() {
    try {
      await load();
      fatal = null;
    } catch (e) {
      if (e.status === 401) return showAuth();
      fatal = e;
    }
    render();
  }

  function notify(text, bad) {
    const n = $("#notice");
    n.textContent = text;
    n.className = "notice" + (bad ? " bad" : "");
    n.hidden = false;
    clearTimeout(noticeTimer);
    noticeTimer = setTimeout(() => (n.hidden = true), 6000);
  }

  async function act(fn) {
    if (busy) return;
    busy = true;
    document.body.classList.add("busy");
    try {
      await fn();
    } catch (e) {
      if (e.status === 401) {
        busy = false;
        document.body.classList.remove("busy");
        return showAuth();
      }
      if (e.status >= 500 || !e.status) fatal = e;
      else notify(e.message.charAt(0).toUpperCase() + e.message.slice(1).replace(/\.$/, "") + ".", true);
    } finally {
      busy = false;
      document.body.classList.remove("busy");
    }
    await refresh();
  }

  // ---------- chart (inline SVG, values from the API) ----------
  function renderForecastChart(forecast, container, opts = {}) {
    const points = (forecast.points || []).map((p) => ({ day: p.day, date: p.date, balance: p.balance_cents / 100 }));
    if (points.length < 2) {
      container.innerHTML = '<p class="muted">No forecast yet.</p>';
      return;
    }
    const sf = forecast.shortfall;
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

    const last = points.length - 1;
    const desc = sf
      ? `Projected balance over the next ${last} days. It starts at ${eur(points[0].balance * 100)}, first goes below zero on ${shortDate(points[sf.first_negative_day].date)}, and reaches its lowest point of ${eur(sf.lowest_balance_cents)} on ${shortDate(points[sf.lowest_day].date)}.`
      : `Projected balance over the next ${last} days. It starts at ${eur(points[0].balance * 100)} and stays at or above zero.`;

    let marks = "";
    (opts.events || []).forEach((e, i) => {
      const p = points[e.day];
      if (!p) return;
      marks += `<circle cx="${x(e.day)}" cy="${y(p.balance)}" r="11" fill="${e.amount_cents > 0 ? "#0f766e" : "#0f3a40"}" stroke="#fff" stroke-width="2"/><text class="ev" x="${x(e.day)}" y="${y(p.balance) + 4}" text-anchor="middle">${i + 1}</text>`;
    });
    let lowDot = "";
    let lowLabel = "";
    if (sf) {
      const lx = x(sf.lowest_day);
      const ly = y(sf.lowest_balance_cents / 100);
      const anchor = lx > W * 0.55 ? "end" : "start";
      lowDot = `<circle cx="${lx}" cy="${ly}" r="5" fill="#b3401b"/>`;
      lowLabel = `<text class="lbl-bad" x="${anchor === "end" ? lx - 8 : lx + 8}" y="${ly - 8}" text-anchor="${anchor}">Day ${sf.lowest_day}: ${eur(sf.lowest_balance_cents)}</text>`;
    }
    const ticks = [0, 10, 20, 30]
      .filter((d) => d <= last)
      .map((d) => `<text x="${x(d)}" y="${H - 6}" text-anchor="${d === 0 ? "start" : d === last ? "end" : "middle"}">${d === 0 ? "Today" : shortDate(points[d].date)}</text>`)
      .join("");
    const top = Math.max(...bal);
    const yTicks = [...new Set([top > 0 ? top : 0, 0])]
      .map((v) => `<text x="${m.l - 6}" y="${y(v) + 4}" text-anchor="end">${eur(Math.round(v / 100) * 10000)}</text>`)
      .join("");

    container.innerHTML = `<svg class="chart" viewBox="0 0 ${W} ${H}" role="img" aria-label="${esc(desc)}">
      <defs>
        <pattern id="hatch-${id}" width="6" height="6" patternUnits="userSpaceOnUse" patternTransform="rotate(45)"><rect width="6" height="6" fill="#fbede7"/><line x1="0" y1="0" x2="0" y2="6" stroke="#b3401b" stroke-width="2"/></pattern>
        <clipPath id="above-${id}"><rect x="0" y="0" width="${W}" height="${y0}"/></clipPath>
        <clipPath id="below-${id}"><rect x="0" y="${y0}" width="${W}" height="${Math.max(H - y0, 0)}"/></clipPath>
      </defs>
      <path d="${area}" fill="#e1f0ee" clip-path="url(#above-${id})"/>
      <path d="${area}" fill="url(#hatch-${id})" clip-path="url(#below-${id})"/>
      <line x1="${m.l}" x2="${W - m.r}" y1="${y0}" y2="${y0}" stroke="#3b5f64" stroke-width="1.5"/>
      <path d="${line}" fill="none" stroke="#0f3a40" stroke-width="2.5" stroke-linejoin="round"/>
      ${yTicks}${ticks}${lowDot}${marks}${lowLabel}
    </svg>`;
  }

  // ---------- chat ----------
  function cardHtml(m) {
    const c = cardOf(m);
    if (!c) return "";
    const inv = m.invoice;
    switch (c.type) {
      case "invoice": {
        if (!inv) return "";
        const simulated = inv.payment_link_url && inv.payment_link_url.includes("/pay/");
        return `<div class="paper"><div class="row"><h3>${esc(inv.number)} · ${esc(inv.counterparty_name)}</h3>${statusHtml(inv.display_status)}</div>
          <div class="amount">${eur(inv.amount_cents)}</div><p>${esc(inv.description)}</p>
          <ul><li>✓ Sent via Jinius with a payment link${simulated ? " (simulated)" : ""}</li>${emailLine(s.messages.find((x) => x.invoice_id === inv.id && x.kind === "invoice"), "Copy")}<li>Due ${longDate(inv.due_date)}</li></ul>
          ${inv.pay_token ? `<div><button type="button" class="btn small" data-act="client" data-token="${esc(inv.pay_token)}">Open the client's page</button></div>` : ""}</div>`;
      }
      case "reminder": {
        const msg = messageById(c.message_id);
        const who = inv ? inv.counterparty_name : "the client";
        const label = c.tone ? TONE_LABEL[c.tone] : "Friendly nudge";
        return `<div class="paper"><div class="row"><h3>Reminder to ${esc(who)}</h3><span class="tag">${esc(label)} · email${msg ? " · " + esc(msg.status) : ""}</span></div>
          ${msg ? `<p class="quote">${esc(msg.body)}</p>` : ""}${msg && msg.status !== "sent" ? `<ul>${emailLine(msg, "Reminder")}</ul>` : ""}</div>`;
      }
      case "warning": {
        const still = s.dash && s.dash.forecast && s.dash.forecast.shortfall;
        const acts = still
          ? `<div class="acts"><button type="button" class="btn primary" data-act="offer">Get ${eur(still.shortfall_cents)} now</button><button type="button" class="btn" data-act="remind-all">Remind all clients first</button></div>`
          : `<p class="done">Covered: no shortfall in the next 30 days now.</p>`;
        return `<div class="alertbox"><strong>In ${plural(c.lowest_day, "day")} you'll be ${eur(c.shortfall_cents)} negative.</strong>${acts}</div>`;
      }
      case "offer": {
        const o = s.offers[c.offer_id];
        if (!o) return "";
        const backing = o.invoices.length
          ? `<ul>${o.invoices.map((i) => `<li>${esc(i.number)} · ${esc(i.client)} · ${eur(i.outstanding_cents)}</li>`).join("")}</ul>`
          : "<p>As an overdraft.</p>";
        const state =
          o.status === "offered"
            ? `<button type="button" class="btn primary" data-act="accept" data-id="${esc(o.id)}">Accept ${eur(o.amount_cents)}</button>`
            : `<p class="done">${o.status === "disbursed" ? "Accepted. The money is in your account (simulated)." : "This offer is " + esc(o.status) + ". Ask again for a new one."}</p>`;
        return `<div class="paper offer"><div class="row"><h3>${eur(o.amount_cents)} ${o.product === "overdraft" ? "overdraft" : "against your unpaid invoices"}</h3><span class="tag">Sample fee ${eur(o.fee_cents)}</span></div>
          ${backing}<p class="muted small">${esc(o.note)}</p>${state}</div>`;
      }
      case "financed":
        return `<div class="okbox"><h3>${eur(c.amount_cents)} advanced against your unpaid invoices</h3><p class="muted">Simulated. A real offer shows the bank's fee and repayment terms before you accept.</p></div>`;
      case "paid":
        return inv ? `<div class="okbox"><h3>${esc(inv.number)} paid: ${eur(inv.amount_cents)}</h3></div>` : "";
      case "suggestions":
        return `<div class="chips">${(c.chips || []).map((t) => `<button type="button" class="chip" data-act="fill" data-text="${esc(t)}">${esc(t)}</button>`).join("")}</div>`;
    }
    return "";
  }

  function msgHtml(m) {
    if (m.role === "system") return `<div class="sys">${esc(m.text)}</div>`;
    if (m.role === "owner") return `<div class="msg me"><p>${esc(m.text)}</p></div>`;
    return `<div class="msg"><span class="avatar" aria-hidden="true">κ</span><div class="body"><p>${esc(m.text)}</p>${cardHtml(m)}</div></div>`;
  }

  function renderChat() {
    const log = $("#chat-log");
    const welcome = s.chat.length
      ? ""
      : msgHtml({ role: "assistant", text: "Tell me who to invoice and I'll send it with a payment link.", card: { type: "suggestions", chips: ["Invoice Kostas €120", "Invoice Kostas, 3 hours at €40"] } });
    log.innerHTML = welcome + s.chat.map(msgHtml).join("");
    log.scrollTop = log.scrollHeight;
    const last = s.chat[s.chat.length - 1];
    if (last && last.id !== lastAnnounced) {
      lastAnnounced = last.id;
      $("#announce").textContent = (last.role === "owner" ? "You: " : "") + last.text;
    }
  }

  // ---------- views ----------
  function remindButton(inv, label) {
    const open = inv.direction === "receivable" && inv.status === "sent" && inv.outstanding_cents > 0 && !inv.is_financed;
    return open ? `<button type="button" class="btn small" data-act="remind" data-id="${esc(inv.id)}" aria-label="Remind ${esc(inv.counterparty_name)} now">${label || "Remind now"}</button>` : "";
  }

  function renderInvoices() {
    const sentCount = (id) => s.messages.filter((m) => m.invoice_id === id && (m.kind === "overdue" || m.kind === "upcoming") && m.status === "sent").length;
    const list = s.invoices.filter((i) => i.direction === invTab);
    const rows = list
      .map((inv) => {
        const n = sentCount(inv.id);
        const client = inv.direction === "receivable" && inv.pay_token && inv.status !== "draft"
          ? `<button type="button" class="btn small" data-act="client" data-token="${esc(inv.pay_token)}">Client view</button>` : "";
        return `<tr><td>${esc(inv.number)}</td><td class="wrap"><strong>${esc(inv.counterparty_name)}</strong><br><span class="muted">${esc(inv.description)}</span></td>
          <td class="r num">${eur(inv.amount_cents)}</td><td class="r num">${inv.outstanding_cents ? eur(inv.outstanding_cents) : "—"}</td>
          <td>${shortDate(inv.issue_date)}</td><td>${shortDate(inv.due_date)}</td><td>${statusHtml(inv.display_status)}</td>
          <td>${inv.direction === "receivable" ? (n ? plural(n, "reminder") : "None") : ""}</td><td><div class="acts-cell">${remindButton(inv)}${client}</div></td></tr>`;
      })
      .join("");
    const tab = (dir, label) => `<button type="button" role="tab" class="tab" aria-selected="${invTab === dir}" data-act="tab" data-dir="${dir}">${label} <span class="muted">(${s.invoices.filter((i) => i.direction === dir).length})</span></button>`;
    $("#view-invoices").innerHTML = `<div class="page-head"><h1>Invoices</h1><p class="muted">Every invoice, newest first. Status, amounts and reminders come from the backend.</p></div>
      <div class="tabs" role="tablist">${tab("receivable", "From the company")}${tab("payable", "To the company")}</div>
      <table><thead><tr><th scope="col">No.</th><th scope="col">${invTab === "receivable" ? "Client" : "Supplier"}</th><th scope="col" class="r">Amount</th><th scope="col" class="r">Still owed</th>
      <th scope="col">Issued</th><th scope="col">Due</th><th scope="col">Status</th><th scope="col">${invTab === "receivable" ? "Reminders" : ""}</th><th scope="col"><span class="sr">Actions</span></th></tr></thead>
      <tbody>${rows || `<tr><td colspan="9" class="muted">No invoices here yet.</td></tr>`}</tbody></table>`;
  }

  function renderDebtors() {
    const list = s.dash.debtors || [];
    const cards = list
      .map((d) => `<div class="debtor"><div><h3>${esc(d.name)}</h3></div><div class="big">${eur(d.outstanding_cents)}</div>
        <button type="button" class="btn small" data-act="remind" data-id="${esc(d.oldest_invoice_id)}" aria-label="Remind ${esc(d.name)} now">Remind now</button>
        <div class="meta">${plural(d.open_invoices, "open invoice")} · oldest ${plural(d.oldest_age_days, "day")} old${d.max_days_overdue ? ` · ${plural(d.max_days_overdue, "day")} overdue` : ""} · reminders sent: ${d.reminders_sent} · last tone: ${d.last_tone ? TONE_LABEL[d.last_tone] : "none yet"}</div></div>`)
      .join("");
    $("#view-debtors").innerHTML = `<div class="page-head"><h1>${list.length ? `${eur(s.dash.owed_cents)} owed by ${plural(list.length, "client")}` : "Nobody owes you anything"}</h1>
      <p class="muted">Sorted by what you're owed. Financed invoices are left out: Kerdos doesn't chase them.</p></div>
      <div class="debtor-list">${cards || "<p>All invoices are paid or financed. New invoices will show up here.</p>"}</div>`;
  }

  function renderForecastView() {
    const f = s.dash.forecast;
    const events = f.events || [];
    const items = events
      .map((e, i) => `<li class="${e.amount_cents > 0 ? "in" : "out"}"><span class="dot" aria-hidden="true">${i + 1}</span><span><strong>${shortDate(e.date)}</strong> · ${esc(e.label)} · <span class="num">${e.amount_cents > 0 ? "+" : "−"}${eur(Math.abs(e.amount_cents))}</span></span></li>`)
      .join("");
    const sf = f.shortfall;
    $("#view-forecast").innerHTML = `<div class="forecast-wrap"><div class="page-head"><h1>${esc(f.headline)}</h1>
      <p class="muted">${esc(f.method || "")} Balance today: ${eur(f.start_balance_cents)}.</p></div>
      <div id="chart-big"></div><ol class="events" aria-label="Payments in and out">${items}</ol>
      ${sf ? `<div><button type="button" class="btn primary" data-act="offer">Get ${eur(sf.shortfall_cents)} now</button></div>` : ""}</div>`;
    renderForecastChart(f, $("#chart-big"), { width: 800, height: 320, id: "big", events });
  }

  function renderReminders() {
    const st = s.settings;
    if (!st) return;
    const order = ["polite", "firm", "final"];
    const tpl = (kind, tone) => (st.templates || []).find((t) => t.kind === kind && (t.tone || "") === (tone || ""));
    const steps = (st.tone_steps || order)
      .map((tone, i, arr) => {
        const t = tpl("overdue", tone);
        const when = i === 0 ? `${plural(st.first_after_due_days, "day")} after the due date` : `${plural(st.every_days, "day")} after the previous one`;
        const name = i === arr.length - 1 ? `Reminder ${i + 1} and after` : `Reminder ${i + 1}`;
        return `<li><span class="n" aria-hidden="true">${i + 1}</span><div><h3>${name}: ${esc(TONE_LABEL[tone].toLowerCase())}</h3><p class="muted">${when}</p>
          ${t ? `<div class="sample"><p><strong>${esc(t.subject)}</strong></p><p>${esc(t.body)}</p></div>` : ""}</div></li>`;
      })
      .join("");
    const sent = s.messages
      .filter((m) => m.kind === "overdue" || m.kind === "upcoming" || m.kind === "invoice")
      .slice(0, 12)
      .map((m) => `<tr><td>${shortDate(m.business_date)}</td><td>${esc(m.kind === "overdue" ? TONE_LABEL[m.tone] + " reminder " + m.sequence : m.kind === "upcoming" ? "Friendly nudge" : "Invoice")}</td>
        <td>${esc(m.invoice_number)}</td><td>${esc(m.recipient)}</td>
        <td class="wrap">${m.status === "failed" ? `<span class="bad-line">failed: ${esc(problem(m.error))}</span> <button type="button" class="btn small" data-act="retry-email" data-id="${esc(m.id)}">Retry</button>`
          : m.status === "cancelled" ? `not sent: ${esc(problem(m.error))}` : esc(m.status)}${m.mode === "manual" ? " · manual" : ""}</td></tr>`)
      .join("");
    $("#view-reminders").innerHTML = `<div class="page-head"><h1>Reminders</h1><p class="muted">Kerdos chases unpaid invoices for you, by email. Financed and paid invoices are never chased.</p>
      <div class="interval"><span class="muted">First reminder</span><strong class="num">${plural(st.first_after_due_days, "day")}</strong><span class="muted">after the due date, then every</span><strong class="num">${plural(st.every_days, "day")}</strong></div></div>
      <ol class="steps">${steps}</ol>
      <h2 class="section-title">Emails sent</h2>
      <table><thead><tr><th scope="col">Day</th><th scope="col">What</th><th scope="col">Invoice</th><th scope="col">To</th><th scope="col">Status</th></tr></thead>
      <tbody>${sent || `<tr><td colspan="5" class="muted">Nothing sent yet.</td></tr>`}</tbody></table>`;
  }

  function renderSide() {
    const f = s.dash.forecast;
    const sf = f.shortfall;
    const owed = s.dash.debtors || [];
    const acts = (s.dash.actions || [])
      .map((a) => (a.type === "financing"
        ? `<button type="button" class="btn primary" data-act="offer">${esc(a.label)}</button>`
        : `<button type="button" class="btn small" data-act="remind" data-id="${esc(a.invoice_id)}">${esc(a.label)}</button>`))
      .join("");
    $("#side-panel").innerHTML = `
      <section><h2>Cash outlook</h2>
        <p class="headline ${sf ? "bad" : ""}">${esc(f.headline)}</p>
        <div id="chart-side"></div></section>
      <section><h2>Who owes you</h2>${owed.length ? `<ul class="owe">${owed.map((d) => `<li><span>${esc(d.name)}</span><span class="num">${eur(d.outstanding_cents)}</span></li>`).join("")}</ul>` : '<p class="muted">Nobody. All invoices are paid or financed.</p>'}</section>
      <section><h2>Action plan</h2><div class="plan">${acts || '<p class="muted">Nothing to do right now.</p>'}</div></section>`;
    renderForecastChart(f, $("#chart-side"), { id: "side" });
  }

  async function loadClient() {
    if (!clientToken) {
      const pick = (list) => list.find((i) => i.source === "chat" && i.status === "sent") || list.find((i) => i.status === "sent") || list[0];
      const inv = pick(s.invoices.filter((i) => i.direction === "receivable" && i.pay_token && i.status !== "draft" && i.status !== "cancelled"));
      clientToken = inv ? inv.pay_token : null;
    }
    s.pay = null;
    s.payError = null;
    if (!clientToken) return;
    try {
      s.pay = await api("GET", "/api/pay/" + encodeURIComponent(clientToken));
    } catch (e) {
      s.payError = e.message;
    }
  }

  function renderClient() {
    const p = s.pay;
    if (!p) {
      $("#view-client").innerHTML = `<p class="client-note">${esc(s.payError || "No invoice sent yet. Create one in Chat and it will show up here.")}</p>`;
      return;
    }
    const r = p.latest_reminder;
    const banner = r ? `<div class="banner ${esc(r.tone || "")}"><p><strong>${r.tone ? TONE_LABEL[r.tone] + " reminder" : "Reminder"}</strong></p><p>${esc(r.text)}</p></div>` : "";
    $("#view-client").innerHTML = `<div class="phone">
        <div class="from"><span class="avatar" aria-hidden="true">${esc(p.company_name.charAt(0))}</span>${esc(p.company_name)}</div>
        ${banner}
        <div><p class="muted">Invoice ${esc(p.number)}</p><p class="amount">${eur(p.can_pay ? p.outstanding_cents : p.amount_cents)}</p></div>
        <dl><dt>To</dt><dd>${esc(p.client_name)}</dd><dt>For</dt><dd>${esc(p.description)}</dd><dt>Due</dt><dd>${longDate(p.due_date)}</dd><dt>Status</dt><dd>${statusHtml(p.status)}</dd></dl>
        ${p.can_pay
          ? `<button type="button" class="btn primary" data-act="pay">Pay ${eur(p.outstanding_cents)} now</button>`
          : `<div class="okbox"><h3>Paid, thank you</h3><p class="muted">${esc(p.client_name)} gets a receipt by email.</p></div>`}
      </div>
      <p class="client-note">This is what ${esc(p.client_name)} sees on their phone. The payment is simulated; with BoC it would go through the client's 1Bank.</p>`;
  }

  function renderError() {
    const fileHint = location.protocol === "file:"
      ? "<p>This page was opened as a file. It needs the Kerdos API on the same address: start the backend and open <a href=\"http://localhost:8080/mockup/\">http://localhost:8080/mockup/</a>.</p>"
      : `<p>Start the backend (see the repo's README), then <button type="button" class="btn small" data-act="retry">try again</button>.</p>`;
    $("#view-error").innerHTML = `<div class="page-head"><h1>Can't reach the Kerdos API</h1><p class="muted">${esc(fatal && fatal.message)}</p></div>${fileHint}`;
  }

  function render() {
    const showing = fatal ? "error" : view;
    $("#app").dataset.view = fatal ? "error" : view;
    document.querySelectorAll(".nav [data-view]").forEach((b) => (b.getAttribute("data-view") === view ? b.setAttribute("aria-current", "page") : b.removeAttribute("aria-current")));
    document.querySelectorAll(".main > section").forEach((sec) => (sec.hidden = sec.id !== `view-${showing}`));
    $("#btn-client-view").textContent = view === "client" ? "Back to owner app" : "Client view";
    if (fatal) {
      renderError();
      return;
    }
    const d = s.dash;
    $("#biz").textContent = d.company;
    $("#badge").hidden = !d.demo;
    $("#btn-skip-time").hidden = !d.demo;
    $("#btn-reset").hidden = !d.demo;
    $("#today").textContent = longDate(d.today) + (d.clock_offset_days ? ` · day ${d.clock_offset_days}` : "");
    renderChat();
    renderInvoices();
    renderDebtors();
    renderForecastView();
    renderReminders();
    renderSide();
    if (view === "client") renderClient();
  }

  // ---------- actions ----------
  const sendChat = (text) => act(async () => {
    view = "chat";
    await api("POST", "/api/chat", { text });
  });

  function showView(name) {
    view = name;
    if (name === "client") {
      loadClient().then(render);
      return;
    }
    render();
  }

  document.addEventListener("click", (e) => {
    const el = e.target.closest("[data-act], [data-view]");
    if (!el) return;
    if (el.dataset.view) return showView(el.dataset.view);
    const a = el.dataset.act;
    if (a === "fill") {
      const input = $("#chat-input");
      input.value = el.dataset.text;
      input.focus();
    } else if (a === "offer") {
      sendChat("Get money now");
    } else if (a === "remind-all") {
      sendChat("Remind everyone");
    } else if (a === "accept") {
      act(async () => {
        await api("POST", "/api/financing/offers/" + el.dataset.id + "/accept");
        view = "chat";
      });
    } else if (a === "remind") {
      act(async () => {
        const m = await api("POST", "/api/invoices/" + el.dataset.id + "/remind");
        const what = `Reminder ${m.invoice_number ? "for " + m.invoice_number + " " : ""}(${m.tone ? TONE_LABEL[m.tone].toLowerCase() : "friendly nudge"})`;
        if (m.status === "sent") notify(`${what} sent to ${m.recipient}.`);
        else if (m.status === "failed") notify(`${what} failed: ${problem(m.error)}.`, true);
        else notify(`${what} is on its way.`);
      });
    } else if (a === "retry-email") {
      act(async () => {
        const m = await api("POST", "/api/messages/" + el.dataset.id + "/retry");
        if (m.status === "sent") notify(`Email sent to ${m.recipient}.`);
        else if (m.status === "failed") notify(`Still failing: ${problem(m.error)}.`, true);
        else notify(`Email ${m.status === "cancelled" ? "not sent: " + problem(m.error) : "is on its way"}.`);
      });
    } else if (a === "client") {
      clientToken = el.dataset.token;
      showView("client");
    } else if (a === "pay") {
      act(async () => {
        s.pay = await api("POST", "/api/pay/" + encodeURIComponent(clientToken));
      });
    } else if (a === "tab") {
      invTab = el.dataset.dir;
      renderInvoices();
    } else if (a === "retry") {
      refresh();
    }
  });

  $("#chat-form").addEventListener("submit", (e) => {
    e.preventDefault();
    const input = $("#chat-input");
    const text = input.value.trim();
    if (!text) return;
    input.value = "";
    sendChat(text);
  });
  $("#btn-skip-time").addEventListener("click", () => act(async () => {
    view = "chat";
    await api("POST", "/api/demo/skip", { days: 3 });
  }));
  $("#btn-reset").addEventListener("click", () => act(async () => {
    await api("POST", "/api/demo/reset");
    view = "chat";
    clientToken = null;
    s.offers = {};
    notify("Demo reset: back to the start of the story.");
  }));
  $("#btn-client-view").addEventListener("click", () => {
    if (view === "client") return showView("chat");
    clientToken = null;
    showView("client");
  });

  // ---------- signing in (only when the API runs with KERDOS_AUTH=accounts) ----------
  const googleHref = (invite) => {
    const q = new URLSearchParams({ return_to: location.origin + location.pathname });
    if (invite) q.set("invite", invite);
    return "/api/auth/google/start?" + q;
  };

  // mode: "signin", or "signup" (the first account, or an invitation).
  async function showAuth(mode, invite) {
    $("#app").hidden = true;
    const box = $("#auth");
    box.hidden = false;
    try {
      auth = await api("GET", "/api/auth/status");
    } catch (e) {
      box.innerHTML = `<div class="auth-card"><h1>Can't reach the Kerdos API</h1><p class="muted">${esc(e.message)}</p></div>`;
      return;
    }
    if (auth.mode !== "accounts" || auth.user) return enterApp();
    let info = null;
    if (invite) {
      try {
        info = await api("GET", "/api/auth/invites/" + encodeURIComponent(invite));
      } catch (e) {
        authNote = e.message;
        invite = "";
      }
    }
    if (!mode) mode = info || auth.needs_setup ? "signup" : "signin";
    if (mode === "signup" && !info && !auth.needs_setup) mode = "signin";
    const note = authNote ? `<p class="auth-note" role="alert">${esc(authNote)}</p>` : "";
    authNote = "";
    const google = auth.google ? `<a class="btn" href="${esc(googleHref(invite))}">Continue with Google</a><p class="auth-or">or ${mode === "signup" ? "choose a password" : "with your email"}</p>` : "";
    let head, form, foot;
    if (mode === "signup") {
      head = info
        ? `<h1>Join ${esc(info.company)}</h1><p class="muted">${esc(info.invited_by || "The owner")} invited you. The invitation is for ${esc(info.email)}.</p>`
        : `<h1>Create the owner account</h1><p class="muted">You're the first one here, so this account owns the business.</p>`;
      form = `<form id="auth-form" data-mode="signup">
          <input type="hidden" name="invite" value="${esc(invite || "")}">
          <label>Your name<input name="name" autocomplete="name" required maxlength="100" value="${esc((info && info.name) || "")}"></label>
          <label>Email<input name="email" type="email" autocomplete="email" required ${info ? `readonly value="${esc(info.email)}"` : ""}></label>
          <label>Password<input name="password" type="password" autocomplete="new-password" required minlength="8"></label>
          <button type="submit" class="btn primary">Create account</button>
        </form>`;
      foot = `<p class="muted">Already have an account? <button type="button" class="auth-link" data-auth="signin">Sign in</button></p>`;
    } else {
      head = `<h1>Sign in</h1>`;
      form = `<form id="auth-form" data-mode="signin">
          <label>Email<input name="email" type="email" autocomplete="email" required></label>
          <label>Password<input name="password" type="password" autocomplete="current-password" required></label>
          <button type="submit" class="btn primary">Sign in</button>
        </form>`;
      foot = auth.needs_setup
        ? `<p>Nobody has an account yet. <button type="button" class="auth-link" data-auth="signup">Create the owner account</button></p>`
        : `<p class="muted">New here? Ask the owner of the business to invite you.</p>`;
    }
    box.innerHTML = `<div class="brand"><span class="mark" aria-hidden="true">κ</span><span class="wordmark">Kerdos</span></div>
      <div class="auth-card">${head}${note}${google}${form}${foot}</div>`;
    const first = box.querySelector("input:not([type=hidden]):not([readonly])");
    if (first) first.focus();
  }

  $("#auth").addEventListener("click", (e) => {
    const el = e.target.closest("[data-auth]");
    if (el) showAuth(el.dataset.auth);
  });

  $("#auth").addEventListener("submit", async (e) => {
    e.preventDefault();
    const form = e.target;
    const f = Object.fromEntries(new FormData(form));
    const button = form.querySelector("button[type=submit]");
    button.disabled = true;
    try {
      if (form.dataset.mode === "signup") await api("POST", "/api/auth/signup", f);
      else await api("POST", "/api/auth/login", { email: f.email, password: f.password });
      await enterApp();
    } catch (err) {
      authNote = err.message;
      await showAuth(form.dataset.mode, f.invite);
      const email = $("#auth input[name=email]");
      if (email && !email.readOnly) email.value = f.email || "";
    }
  });

  async function enterApp() {
    $("#auth").hidden = true;
    $("#app").hidden = false;
    try {
      auth = await api("GET", "/api/auth/status");
    } catch (_) {
      /* refresh() below shows the error */
    }
    const signedIn = auth && auth.mode === "accounts" && auth.user;
    $("#user-name").hidden = !signedIn;
    $("#user-name").textContent = signedIn ? auth.user.name : "";
    $("#btn-sign-out").hidden = !signedIn;
    await refresh();
  }

  $("#btn-sign-out").addEventListener("click", async () => {
    try {
      await api("POST", "/api/auth/logout");
    } catch (_) {
      /* signed out here anyway */
    }
    s.offers = {};
    showAuth("signin");
  });

  // Coming back from Google: ?login_code=… signs in; ?auth_message=… is a problem to show.
  async function start() {
    const q = new URLSearchParams(location.search);
    const code = q.get("login_code");
    const invite = q.get("invite") || "";
    if (q.get("auth_message")) authNote = q.get("auth_message");
    if (code || q.has("auth_error")) history.replaceState(null, "", location.pathname);
    if (code) {
      try {
        await api("POST", "/api/auth/exchange", { code });
      } catch (e) {
        authNote = e.message;
      }
    }
    try {
      auth = await api("GET", "/api/auth/status");
    } catch (e) {
      fatal = e;
      render();
      return;
    }
    if (auth.mode === "accounts" && (!auth.user || invite)) return showAuth(undefined, invite);
    await enterApp();
  }

  start();
})();
