import type {
  Account,
  AuthStatus,
  ChatMessage,
  Dashboard,
  Debtor,
  Direction,
  Forecast,
  Invoice,
  Message,
  Offer,
  Invite,
  InviteInfo,
  ReminderPolicy,
  SignedIn,
  Team,
} from "./types";

// The Go backend's API (backend/README.md, "Endpoints"). Every backend call goes through this.
export interface ApiClient {
  // Signing in. These work without a session.
  getAuthStatus(): Promise<AuthStatus>;
  signUp(input: { name: string; email: string; password: string; invite?: string }): Promise<SignedIn>;
  logIn(email: string, password: string): Promise<SignedIn>;
  logOut(): Promise<void>;
  exchangeLoginCode(code: string): Promise<SignedIn>;
  getInvite(token: string): Promise<InviteInfo>;
  startGoogle(returnTo: string, invite?: string): Promise<string>;

  // The account page.
  getMe(): Promise<Account>;
  updateMe(input: { name?: string; email?: string; current_password?: string }): Promise<Account>;
  changePassword(currentPassword: string, newPassword: string): Promise<void>;
  linkGoogle(returnTo: string): Promise<string>;
  unlinkGoogle(): Promise<Account>;
  logOutEverywhere(): Promise<void>;

  // The team (owner only).
  getTeam(): Promise<Team>;
  invite(email: string, name?: string): Promise<{ invite: Invite; link: string }>;
  revokeInvite(id: string): Promise<void>;
  removeMember(id: string): Promise<void>;

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
