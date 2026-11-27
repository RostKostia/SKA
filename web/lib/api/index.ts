import { cookies } from "next/headers";
import type { ApiClient } from "./client";
import { apiBaseUrl, SESSION_COOKIE } from "./config";
import { createHttpApi } from "./http";

// The only entry point server code uses to reach the Go backend.
export function getApi(): ApiClient {
  return createHttpApi({
    baseUrl: apiBaseUrl(process.env),
    getCookie: async () => {
      const session = (await cookies()).get(SESSION_COOKIE);
      return session ? `${SESSION_COOKIE}=${session.value}` : undefined;
    },
  });
}
