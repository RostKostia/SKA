type Env = Record<string, string | undefined>;

const DEFAULT_API_URL = "http://127.0.0.1:8080";

// Name of the session cookie forwarded to the backend once Google sign-in exists there.
export const SESSION_COOKIE = "kerdos_session";

// Where the Go backend listens (KERDOS_API_URL), without a trailing slash.
export function apiBaseUrl(env: Env): string {
  const raw = env.KERDOS_API_URL || DEFAULT_API_URL;
  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    throw new Error(`KERDOS_API_URL must be an http(s) URL, got "${raw}"`);
  }
  if (url.protocol !== "http:" && url.protocol !== "https:") {
    throw new Error(`KERDOS_API_URL must be an http(s) URL, got "${raw}"`);
  }
  return raw.replace(/\/+$/, "");
}
