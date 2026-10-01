import "server-only";

import { cookies } from "next/headers";
import { redirect } from "next/navigation";

import type { components } from "@/lib/api/schema";

import { ACCESS_COOKIE, API_URL } from "./config";
import { verifyAccessToken, type Session } from "./jwt";
import { hasAnyRole, type Role } from "./roles";

export type Me = components["schemas"]["Me"];

export interface ServerSession extends Session {
  token: string;
}

// getSession reads and verifies the access token cookie. src/proxy.ts has
// already refreshed it if it had expired.
export async function getSession(): Promise<ServerSession | null> {
  const token = (await cookies()).get(ACCESS_COOKIE)?.value;
  if (!token) return null;
  const session = await verifyAccessToken(token);
  return session ? { ...session, token } : null;
}

export async function requireSession(): Promise<ServerSession> {
  const session = await getSession();
  if (!session) redirect("/login");
  return session;
}

// canAccess reports whether the signed-in user holds one of roles. Pages call
// it and render <Forbidden /> when it is false.
export async function canAccess(roles: readonly Role[]): Promise<boolean> {
  const session = await requireSession();
  return hasAnyRole(session.roles, roles);
}

export type MeResult = { ok: true; me: Me } | { ok: false; status: number };

export async function fetchMe(token: string): Promise<MeResult> {
  try {
    const res = await fetch(`${API_URL}/v1/me`, {
      headers: { authorization: `Bearer ${token}` },
      cache: "no-store",
    });
    if (!res.ok) return { ok: false, status: res.status };
    return { ok: true, me: (await res.json()) as Me };
  } catch {
    return { ok: false, status: 502 };
  }
}
