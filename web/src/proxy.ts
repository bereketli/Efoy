import { NextResponse, type NextRequest } from "next/server";

import { ACCESS_COOKIE, REFRESH_COOKIE } from "@/lib/auth/config";
import { clearSessionCookies, setSessionCookies } from "@/lib/auth/cookies";
import { verifyAccessToken } from "@/lib/auth/jwt";
import { refreshSession } from "@/lib/auth/upstream";

// Runs before every console page and /api/efoy/* call:
//  - a valid access token passes through;
//  - an expired one is refreshed with the refresh cookie, and the new tokens
//    are passed to this request and stored for the next ones;
//  - otherwise pages redirect to /login and API calls get core-api's 401.
export async function proxy(request: NextRequest) {
  const access = request.cookies.get(ACCESS_COOKIE)?.value;
  if (access && (await verifyAccessToken(access))) {
    return NextResponse.next();
  }

  const refresh = request.cookies.get(REFRESH_COOKIE)?.value;
  if (refresh) {
    const result = await refreshSession(request, refresh);
    if (result.ok) {
      request.cookies.set(ACCESS_COOKIE, result.tokens.access_token);
      request.cookies.set(REFRESH_COOKIE, result.tokens.refresh_token);
      const response = NextResponse.next({ request: { headers: request.headers } });
      setSessionCookies(response.cookies, result.tokens);
      return response;
    }
    if (result.reason === "unavailable") {
      // Keep the session; the page shows that the API is unreachable.
      return NextResponse.next();
    }
  }

  const isApi = request.nextUrl.pathname.startsWith("/api/");
  let response: NextResponse;
  if (isApi) {
    response = NextResponse.next();
  } else {
    const login = new URL("/login", request.url);
    const next = request.nextUrl.pathname + request.nextUrl.search;
    if (next !== "/") login.searchParams.set("next", next);
    response = NextResponse.redirect(login);
  }
  if (access || refresh) clearSessionCookies(response.cookies);
  return response;
}

export const config = {
  matcher: ["/((?!login|api/auth|_next/static|_next/image|favicon.ico|.*\\.(?:svg|png|jpg|jpeg|gif|webp|ico)$).*)"],
};
