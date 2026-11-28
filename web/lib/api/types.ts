// JSON shapes of the Go API (backend/internal/app). The endpoint table in backend/README.md is
// the contract. Money is integer cents; dates are "YYYY-MM-DD"; instants are RFC 3339.

export type Direction = "receivable" | "payable";
export type DisplayStatus =
  | "draft"
  | "sent"
  | "paid"
  | "cancelled"
  | "unpaid"
  | "overdue"
  | "partially_paid"
  | "financed";
export type Tone = "polite" | "firm" | "final";

export type User = { id: string; email: string; name: string; role: string };

// The signed-in user as the account and team pages show them. role "admin" is the owner.
export type Account = User & {
  has_password: boolean;
  google_linked: boolean;
  google_email?: string;
  created_at: string;
  last_login_at: string | null;
};

// GET /api/auth/status. mode "dev": every request is the owner, there is no sign-in.
export type AuthStatus = { mode: "dev" | "accounts"; needs_setup: boolean; google: boolean; user: Account | null };

// What signing in returns: the session token is for the web app's own cookie.
export type SignedIn = { user: Account; token: string };

export type InviteInfo = { email: string; name?: string; company: string; invited_by?: string; expires_at: string };

export type Member = Account & { you: boolean };

export type Invite = {
  id: string;
  email: string;
  name?: string;
  role: string;
  invited_by?: string;
  created_at: string;
  expires_at: string;
  expired: boolean;
  email_message_id?: string;
  email_status?: "queued" | "sent" | "failed" | "cancelled";
  email_error?: string;
};

export type Team = { members: Member[]; invites: Invite[] };

export type Invoice = {
  id: string;
  number: string;
  direction: Direction;
  counterparty_id: string;
  counterparty_name: string;
  description: string;
  amount_cents: number;
  vat_cents: number;
  paid_cents: number;
  outstanding_cents: number;
  currency: string;
  issue_date: string;
  due_date: string;
  status: "draft" | "sent" | "paid" | "cancelled";
  display_status: DisplayStatus;
  days_overdue: number;
  is_financed: boolean;
  source: string;
  source_text: string | null;
  payment_link_url: string | null;
  sent_at: string | null;
  paid_at: string | null;
};

export type Debtor = {
  counterparty_id: string;
  name: string;
  open_invoices: number;
  outstanding_cents: number;
  oldest_age_days: number;
  max_days_overdue: number;
  reminders_sent: number;
  last_tone: string | null;
  oldest_invoice_id: string;
};

export type ForecastPoint = { day: number; date: string; balance_cents: number };
export type ForecastEvent = {
  day: number;
  date: string;
  amount_cents: number;
  kind: "receivable" | "payable" | "cashflow";
  label: string;
  invoice_id: string | null;
  cashflow_id: string | null;
};
export type Shortfall = {
  first_negative_day: number;
  lowest_day: number;
  lowest_balance_cents: number;
  shortfall_cents: number;
};
export type Forecast = {
  today: string;
  horizon_days: number;
  start_balance_cents: number;
  points: ForecastPoint[];
  events: ForecastEvent[];
  shortfall: Shortfall | null;
  headline: string;
  method: string;
};

export type Action = { type: "financing" | "remind"; label: string; amount_cents?: number; invoice_id?: string };

export type Dashboard = {
  company: string;
  today: string;
  clock_offset_days: number;
  demo: boolean;
  balance_cents: number | null;
  owed_cents: number;
  forecast: Forecast;
  debtors: Debtor[];
  actions: Action[];
};

export type Offer = {
  id: string;
  product: "invoice_financing" | "overdraft";
  amount_cents: number;
  fee_cents: number;
  repayment_date: string | null;
  status: string;
  provider: string;
  forecast_run_id: string;
  offered_at: string;
  invoices: { invoice_id: string; number: string; client: string; outstanding_cents: number }[];
  note: string;
};

// What the UI draws under a chat message. Other fields depend on the type.
export type ChatCard = {
  type: "invoice" | "reminder" | "warning" | "offer" | "financed" | "paid" | "need_details" | "suggestions" | "debtors" | "ok";
  offer_id?: string;
  amount_cents?: number;
  fee_cents?: number;
  shortfall_cents?: number;
  chips?: string[];
};

export type ChatMessage = {
  id: string;
  role: "owner" | "assistant" | "system";
  text: string;
  intent?: string;
  card?: ChatCard | null;
  // Current state of the card's invoice, if the card has one.
  invoice?: Invoice | null;
  created_at: string;
};

export type Message = {
  id: string;
  kind: string;
  mode: "manual" | "automated";
  invoice_id?: string;
  invoice_number?: string;
  sequence?: number | null;
  tone?: string;
  recipient: string;
  subject: string;
  body: string;
  status: string;
  business_date: string;
  sent_at: string | null;
  provider?: string;
  error?: string;
  created_at: string;
};

export type Template = { id: string; kind: string; tone?: string; language: string; subject: string; body: string };
export type ReminderPolicy = {
  id: string;
  name: string;
  before_due_days: number[];
  first_after_due_days: number;
  every_days: number;
  tone_steps: string[];
  max_reminders: number | null;
  enabled: boolean;
  templates: Template[];
};

export type ApiErrorBody = { error: { code: string; message: string } };
