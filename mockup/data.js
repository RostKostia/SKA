// Sample data only. Swap this file for live Jinius / BoC adapters later.
const KUMBARES_DATA = {
  business: "Nicosia Studio (sample)",
  day: 0,
  balance: 1000,
  reminderEveryDays: 3,
  invoices: [
    {
      id: "inv-1",
      client: "Elena's Bakery",
      amount: 450,
      description: "Menu and logo design",
      sentDay: -10,
      dueDay: 4,
      status: "unpaid",
      reminders: [
        { day: -7, tone: "polite" },
        { day: -4, tone: "firm" },
        { day: -1, tone: "final" },
      ],
    },
    {
      id: "inv-2",
      client: "Andreas Electric",
      amount: 380,
      description: "Website update",
      sentDay: -5,
      dueDay: 9,
      status: "unpaid",
      reminders: [{ day: -2, tone: "polite" }],
    },
    {
      id: "inv-0",
      client: "Cyprus Tiles Ltd",
      amount: 600,
      description: "Brand refresh",
      sentDay: -20,
      dueDay: -6,
      status: "paid",
      reminders: [{ day: -14, tone: "polite" }],
    },
  ],
  inflows: [{ day: 2, label: "Maria Savva, deposit (sample)", amount: 300 }],
  outflows: [
    { day: 4, label: "Studio rent", amount: 1500 },
    { day: 12, label: "Insurance", amount: 600 },
  ],
  financed: false,
};

if (typeof module !== "undefined") module.exports = KUMBARES_DATA;
