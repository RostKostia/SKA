import type {
  ChatMessage,
  Dashboard,
  Debtor,
  Direction,
  Forecast,
  Invoice,
  Message,
  Offer,
  ReminderPolicy,
  User,
} from "./types";

// The Go backend's API (backend/README.md, "Endpoints"). Every backend call goes through this.
export interface ApiClient {
  getMe(): Promise<User>;
  getDashboard(): Promise<Dashboard>;

  listChat(): Promise<ChatMessage[]>;
  sendChat(text: string): Promise<ChatMessage[]>;

  listInvoices(filter?: { direction?: Direction; status?: string }): Promise<Invoice[]>;
  remindInvoice(id: string): Promise<Message>;
  cancelInvoice(id: string): Promise<Invoice>;
  listDebtors(): Promise<Debtor[]>;

  getForecast(horizonDays?: number): Promise<Forecast>;
  createOffer(): Promise<Offer>;
  getOffer(id: string): Promise<Offer>;
  acceptOffer(id: string): Promise<{ offer: Offer; forecast: Forecast }>;

  listMessages(filter?: { invoice_id?: string; kind?: string }): Promise<Message[]>;
  getReminderSettings(): Promise<ReminderPolicy>;
}

export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}
