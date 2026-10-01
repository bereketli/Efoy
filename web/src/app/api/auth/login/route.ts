import { NextResponse, type NextRequest } from "next/server";

import { setSessionCookies } from "@/lib/auth/cookies";
import { hasConsoleAccess } from "@/lib/auth/roles";
import { apiFetch, apiUnavailable, passthrough, problem, type TokenPair } from "@/lib/auth/upstream";

// POST /api/auth/login { email, password, totp_code? }
// Signs in against core-api and stores the tokens in httpOnly cookies. The
// browser only receives the user profile.
export async function POST(req: NextRequest) {
  let body: unknown;
  try {
    body = await req.json();
  } catch {
    return problem(400, "INVALID_JSON", "The request body must be valid JSON.");
  }

  let res: Response;
  try {
    res = await apiFetch(req, "/v1/auth/login", { method: "POST", body: JSON.stringify(body) });
  } catch {
    return apiUnavailable();
  }
  if (!res.ok) return passthrough(res);

  const tokens = (await res.json()) as TokenPair;
  if (!hasConsoleAccess(tokens.user.roles.map((r) => r.role))) {
    // Valid account, but riders, guardians and drivers use the mobile apps.
    await apiFetch(req, "/v1/auth/logout", {
      method: "POST",
      headers: { authorization: `Bearer ${tokens.access_token}` },
    }).catch(() => undefined);
    return problem(403, "NO_CONSOLE_ACCESS", "This account cannot use the Efoy console. Use the Efoy app instead.");
  }

  const response = NextResponse.json({ user: tokens.user });
  setSessionCookies(response.cookies, tokens);
  return response;
}
