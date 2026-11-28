# Kerdos: pitch and business model

The 4-minute pitch for the Bank of Cyprus Fintech Hackathon 7.0 (Track #4, Fintech for SMEs & Entrepreneurship), Sunday 29 November 2026, with slides and a live demo. It also sets out how Kerdos makes money for the bank.

**One-liner:** Type one sentence: Kerdos creates the invoice, sends it and keeps reminding until the client pays, right inside your bank's app.

## Run of show (4:00)

Suggested split: **Sasha** speaks the story parts, **Artem** runs the demo and explains the integrations. Artem sits at the laptop from the start, so there is no hand-over pause before the demo.

| # | Section | Time | Ends at | Speaker | Slide |
|---|---|---|---|---|---|
| 1 | Hook | 0:20 | 0:20 | Sasha | One number: "1 in 4" |
| 2 | Problem | 0:30 | 0:50 | Sasha | Late payments in the EU, three stats |
| 3 | Solution | 0:20 | 1:10 | Sasha | The one-liner and three icons: invoice, remind, paid |
| 4 | Live demo | 1:30 | 2:40 | Artem | Live app, second phone shown on screen |
| 5 | Integrations | 0:20 | 3:00 | Artem | BoC API, Jinius, Claude |
| 6 | Business model | 0:30 | 3:30 | Sasha | B2B2C diagram: Bank → Kerdos → SME → client |
| 7 | Future | 0:15 | 3:45 | Sasha | Freelancers on platforms like Fiverr |
| 8 | Close | 0:15 | 4:00 | Sasha | The one-liner and the team |

## Script

Read at a calm pace, about 130 words a minute. Brackets are stage directions, not spoken.

### 1. Hook (0:20), Sasha

> In the European Union, one in four bankruptcies happens for one reason: invoices that were paid late. Not bad products, not a lack of customers. Just money that arrives too late. We are Kerdos, and we make sure small businesses get paid.

### 2. Problem (0:30), Sasha

> Only about half of invoices in the EU are paid on time. Almost half of European companies say late payments hurt them, and the average business-to-business invoice takes about 62 days to be paid. For a café, a plumber or a freelancer in Cyprus, that is two months of rent and salaries paid out of their own pocket. And the tools they use are a Word template, an Excel sheet and awkward phone calls to chase their own clients.

### 3. Solution (0:20), Sasha

> Kerdos is an AI assistant inside your bank's app. You type one sentence. Kerdos creates the invoice, sends it with a payment link, and keeps reminding the client, politely and then firmly, until they pay. Artem will show you.

### 4. Live demo (1:30), Artem

[Before: `KERDOS_BANK=mock`, demo data reset, Kostas pointed at the second phone. See the checklist in the main README.]

1. **Chat (0:20).** [Type "Invoice Kostas €120".]
   > I just type, like I would text a friend: "Invoice Kostas, 120 euros." Claude reads it, finds Kostas in my clients and fills in the invoice.
2. **Invoice emailed (0:20).** [Show the second phone receiving the email.]
   > And here it is, already in Kostas's inbox, on this phone, with a "Pay now" link. No PDF to attach, no email to write.
3. **Automatic reminders (0:25).** [Press Skip time; show the reminder going out and the reminder history.]
   > Now let's jump forward a few days. Another client, Andreas, is late. I didn't do anything: Kerdos sent him a reminder on its own, and each one gets a little firmer. I always see who owes me what.
4. **Client pays (0:15).** [On the second phone, open the link and pay.]
   > Kostas opens the link and pays in two taps.
5. **Paid (0:10).** [Back on the dashboard: the invoice turns Paid, the reminders stop.]
   > The invoice is marked paid, the reminders stop, and the money is in my account. That's it.

Do not show the "Get money now" financing button: financing is not part of the pitch.

### 5. Integrations (0:20), Artem

> Under the hood, Kerdos uses three integrations. The Bank of Cyprus open banking API gives us the balance and matches incoming payments to invoices. Jinius sends the e-invoice and the payment link. And Claude understands what the owner types. Each one sits behind a mock, so the product works even when a sandbox is down.

### 6. Business model (0:30), Sasha

> Our model is business to bank to business. The Bank of Cyprus offers Kerdos free inside its app, so every SME client gets it with one tap. The bank earns a small fee on each invoice paid through a Kerdos payment link, just like card acquiring. And it keeps its business clients: once your invoices and your clients' payments run through your bank, you don't move to another one.

### 7. Future (0:15), Sasha

> We start with all small businesses: freelancers, micro and small companies. Next come freelancers who sell on platforms like Fiverr, so their clients can pay them straight into a Bank of Cyprus account.

### 8. Close (0:15), Sasha

> Type one sentence: Kerdos creates the invoice, sends it and keeps reminding until the client pays, right inside your bank's app. We are Sasha, Artem and Kostia. Thank you.

## Business model

**Type:** B2B2C. The bank is the customer and the distribution channel; small businesses are the users.

**Who pays what**

- **Small business:** pays nothing. Kerdos comes free in the Bank of Cyprus app.
- **Bank of Cyprus:** offers Kerdos to its SME clients and earns from it in two ways:
  1. **A fee on each invoice paid through a Kerdos payment link**, similar to card acquiring. *Working assumption, not yet confirmed by the team:* a small percentage of the invoice amount, kept below card acquiring rates so it is cheaper for the business than taking a card payment.
  2. **Retention and deposits.** Invoices, payments and reminders all run through the business's BoC account, so the money lands at BoC and the business has a reason to stay.

**Why the bank wants it**

- More payment volume through BoC accounts and Jinius.
- SME clients who stay, because their invoicing lives in the bank's app.
- A reason for new small businesses and freelancers to open an account with BoC.

**Target users:** all SMEs: freelancers, micro and small businesses in Cyprus. Later: freelancers selling on platforms like Fiverr.

**Dropped:** financing against unpaid invoices ("Get €800 now") is no longer part of the business model or the pitch.

## Competitors

| Today's option | What it lacks |
|---|---|
| Word or Excel invoices | Manual every time, no payment link, no reminders, nobody tracks who paid |
| Xero, QuickBooks | A separate subscription and a separate app built for accountants, not connected to the business's own bank account in Cyprus |
| Chasing by phone or WhatsApp | Awkward, slow and easy to forget; it damages the client relationship |
| **Kerdos** | One sentence to invoice, automatic reminders, payment straight into the BoC account, free inside the bank's app |

## Facts and sources

| Claim in the pitch | Source |
|---|---|
| 1 in 4 bankruptcies in the EU is due to late payment; only about half of invoices are paid on time | European Commission, Late Payment Regulation factsheet, September 2023: https://single-market-economy.ec.europa.eu/system/files/2023-09/SOTEU_factsheet_2023_Late%20Payments%20Regulation_V6.pdf |
| 47% of EU businesses are affected by payment delays; the average B2B payment takes 61.8 days | EU Payment Observatory, 2024: https://single-market-economy.ec.europa.eu/document/download/d3f9aa5a-c33e-40d0-a867-5c0198ee7a75_en |
| 11% of revenue is paid late; 58% of businesses think AI can help | Intrum, European Payment Report 2025: https://www.intrum.com/insights/publications/epr-2025/ |

## Q&A prep

- **"Why would the bank give it away for free?"** Because the bank earns on the payments, not on a subscription, and every invoice paid through Kerdos lands in a BoC account.
- **"What's the fee exactly?"** It is a working assumption: a small percentage per paid invoice, below card acquiring. The bank would set the final number.
- **"Is the bank integration real?"** The BoC sandbox app is registered and its token works; the demo runs on a mock behind the same interface, so it never depends on the sandbox being up. Jinius registration is submitted and waiting for approval.
- **"Why not Xero?"** Xero is a separate tool for accountants. Kerdos is for the owner, in the app they already open every day, and the money goes straight to their bank account.
- **"What stops a client ignoring the reminders?"** Each reminder gets firmer, and the owner sees exactly who owes what, so they can act early.
