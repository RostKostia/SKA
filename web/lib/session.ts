import { cookies, headers } from "next/headers";
import { SESSION_COOKIE } from "./api/config";

// The team page shows a new invitation's link from this short-lived cookie (path /team).
export const INVITE_LINK_COOKIE = "kerdos_invite_link";

// The session lasts 30 days, like the backend's.
export const SESSION_MAX_AGE = 30 * 24 * 60 * 60;

type Env = Record<string, string | undefined>;

// Where this web app is reachable in the browser: KERDOS_WEB_URL, or the request's own host.
// It must match the backend's KERDOS_PUBLIC_URL, which the backend checks Google's return
// address against.
export function webOrigin(env: Env, requestHeaders: Headers): string {
  if (env.KERDOS_WEB_URL) return env.KERDOS_WEB_URL.replace(/\/+$/, "");
  const host = requestHeaders.get("x-forwarded-host") ?? requestHeaders.get("host") ?? "localhost:3000";
  const proto = requestHeaders.get("x-forwarded-proto") ?? (host.startsWith("localhost") || host.startsWith("127.") ? "http" : "https");
  return `${proto}://${host}`;
}

export function sessionCookie(token: string, origin: string) {
  return {
    name: SESSION_COOKIE,
    value: token,
    httpOnly: true,
    sameSite: "lax" as const,
    path: "/",
    secure: origin.startsWith("https://"),
    maxAge: SESSION_MAX_AGE,
  };
}

// For Server Actions: keep the backend's session token in the web app's own cookie.
export async function startSession(token: string) {
  const origin = webOrigin(process.env, await headers());
  (await cookies()).set(sessionCookie(token, origin));
}

export async function endSession() {
  (await cookies()).delete(SESSION_COOKIE);
}

export async function currentWebOrigin() {
  return webOrigin(process.env, await headers());
}
