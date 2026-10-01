import { NextResponse, type NextRequest } from "next/server";

import type { components } from "@/lib/api/schema";

import { API_URL } from "./config";

export type TokenPair = components["schemas"]["TokenPair"];
export type Problem = components["schemas"]["Problem"];

// apiFetch calls core-api on behalf of the browser request, forwarding the
// caller's IP, user agent and language. It throws if core-api is unreachable.
export function apiFetch(req: NextRequest, path: string, init: RequestInit = {}): Promise<Response> {
  const headers = new Headers(init.headers);
  if (init.body && !headers.has("content-type")) headers.set("content-type", "application/json");
  const forwardedFor = req.headers.get("x-forwarded-for");
  if (forwardedFor) headers.set("x-forwarded-for", forwardedFor);
  for (const name of ["user-agent", "accept-language"]) {
    const value = req.headers.get(name);
    if (value) headers.set(name, value);
  }
  return fetch(`${API_URL}${path}`, { ...init, headers, cache: "no-store" });
}

export function problem(status: number, code: string, detail: string): NextResponse {
  const body: Problem = {
    type: `https://docs.efoy.et/errors/${code.toLowerCase().replaceAll("_", "-")}`,
    title: status >= 500 ? "Service unavailable" : "Request failed",
    status,
    code,
    detail,
  };
  return NextResponse.json(body, { status, headers: { "content-type": "application/problem+json" } });
}

export const apiUnavailable = () =>
  problem(502, "API_UNAVAILABLE", "The Efoy API cannot be reached. Try again in a moment.");

// passthrough relays a core-api response (status, body and the headers the
// browser needs) unchanged.
export async function passthrough(res: Response): Promise<NextResponse> {
  const headers = new Headers();
  for (const name of ["content-type", "retry-after", "cache-control"]) {
    const value = res.headers.get(name);
    if (value) headers.set(name, value);
  }
  return new NextResponse(res.body, { status: res.status, headers });
}

export type RefreshResult = { ok: true; tokens: TokenPair } | { ok: false; reason: "invalid" | "unavailable" };

// refreshSession rotates the refresh token. "unavailable" means core-api could
// not be reached, in which case the caller should keep the cookies.
export async function refreshSession(req: NextRequest, refreshToken: string): Promise<RefreshResult> {
  try {
    const res = await apiFetch(req, "/v1/auth/refresh", {
      method: "POST",
      body: JSON.stringify({ refresh_token: refreshToken }),
    });
    if (res.ok) return { ok: true, tokens: (await res.json()) as TokenPair };
    return { ok: false, reason: res.status >= 500 ? "unavailable" : "invalid" };
  } catch {
    return { ok: false, reason: "unavailable" };
  }
}
