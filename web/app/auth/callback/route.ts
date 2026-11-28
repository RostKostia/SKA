import { NextResponse, type NextRequest } from "next/server";
import { getApi } from "@/lib/api";
import { ApiError } from "@/lib/api/client";
import { flash, safePath } from "@/lib/flash";
import { sessionCookie, webOrigin } from "@/lib/session";

// Where Google sign-in comes back to (by way of the backend). Three outcomes:
// ?login_code=… swap it for a session and go on; ?google=linked (from the account page);
// ?auth_error=…&auth_message=… show the message.
export async function GET(request: NextRequest) {
  const q = request.nextUrl.searchParams;
  const next = safePath(q.get("next"));
  const linking = q.get("link") === "1";
  const origin = webOrigin(process.env, request.headers);
  const go = (path: string) => NextResponse.redirect(new URL(path, origin), 303);

  if (q.get("auth_error")) {
    const message = q.get("auth_message") || "Google sign-in didn't work. Try again.";
    return go(flash(linking ? next : "/login", { error: message }));
  }
  if (q.get("google") === "linked") {
    return go(flash(next, { notice: "Your Google account is linked. You can now sign in with it." }));
  }
  const code = q.get("login_code");
  if (!code) return go("/login");
  try {
    const { token } = await getApi().exchangeLoginCode(code);
    const res = go(next);
    res.cookies.set(sessionCookie(token, origin));
    return res;
  } catch (error) {
    if (!(error instanceof ApiError)) throw error;
    return go(flash("/login", { error: error.message }));
  }
}
