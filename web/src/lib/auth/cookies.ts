import type { NextResponse } from "next/server";

import { ACCESS_COOKIE, REFRESH_COOKIE, SECURE_COOKIES, STAFF_SESSION_SECONDS } from "./config";

type ResponseCookies = NextResponse["cookies"];

export interface SessionTokens {
  access_token: string;
  refresh_token: string;
  expires_in: number;
}

const base = { httpOnly: true, secure: SECURE_COOKIES, sameSite: "lax", path: "/" } as const;

// Tokens live in httpOnly cookies so page scripts can never read them.
export function setSessionCookies(cookies: ResponseCookies, tokens: SessionTokens) {
  cookies.set(ACCESS_COOKIE, tokens.access_token, { ...base, maxAge: tokens.expires_in });
  cookies.set(REFRESH_COOKIE, tokens.refresh_token, { ...base, maxAge: STAFF_SESSION_SECONDS });
}

export function clearSessionCookies(cookies: ResponseCookies) {
  cookies.set(ACCESS_COOKIE, "", { ...base, maxAge: 0 });
  cookies.set(REFRESH_COOKIE, "", { ...base, maxAge: 0 });
}
