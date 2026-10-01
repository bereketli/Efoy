import { NextResponse, type NextRequest } from "next/server";

import { ACCESS_COOKIE } from "@/lib/auth/config";
import { clearSessionCookies } from "@/lib/auth/cookies";
import { apiFetch } from "@/lib/auth/upstream";

// POST /api/auth/logout ends the session in core-api (revoking every refresh
// token of this login) and clears the cookies, even if core-api is down.
export async function POST(req: NextRequest) {
  const access = req.cookies.get(ACCESS_COOKIE)?.value;
  if (access) {
    await apiFetch(req, "/v1/auth/logout", {
      method: "POST",
      headers: { authorization: `Bearer ${access}` },
    }).catch(() => undefined);
  }
  const response = new NextResponse(null, { status: 204 });
  clearSessionCookies(response.cookies);
  return response;
}
