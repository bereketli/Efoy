import type { NextRequest } from "next/server";

import { ACCESS_COOKIE } from "@/lib/auth/config";
import { apiFetch, apiUnavailable, passthrough, problem } from "@/lib/auth/upstream";

type Context = { params: Promise<{ path: string[] }> };

const FORWARDED_HEADERS = ["accept", "content-type", "idempotency-key"];

// /api/efoy/* forwards browser calls from the typed API client to core-api,
// adding the access token from the session cookie.
async function forward(req: NextRequest, { params }: Context) {
  const { path } = await params;
  const target = "/" + path.map(encodeURIComponent).join("/");

  // Token endpoints go through /api/auth/* so tokens never reach page scripts.
  if (target.startsWith("/v1/auth/")) {
    return problem(404, "NOT_FOUND", "Use /api/auth/* for sign-in and sign-out.");
  }

  const headers = new Headers();
  for (const name of FORWARDED_HEADERS) {
    const value = req.headers.get(name);
    if (value) headers.set(name, value);
  }
  const token = req.cookies.get(ACCESS_COOKIE)?.value;
  if (token) headers.set("authorization", `Bearer ${token}`);

  const init: RequestInit = { method: req.method, headers };
  if (req.method !== "GET" && req.method !== "HEAD") init.body = await req.arrayBuffer();

  try {
    return passthrough(await apiFetch(req, target + req.nextUrl.search, init));
  } catch {
    return apiUnavailable();
  }
}

export { forward as DELETE, forward as GET, forward as PATCH, forward as POST, forward as PUT };
