import { createRemoteJWKSet, jwtVerify } from "jose";

import { API_URL, JWT_AUDIENCE, JWT_ISSUER } from "./config";

// core-api publishes its Ed25519 public keys; jose caches them and refetches
// when a token arrives with an unknown key id (after a key rotation).
const jwks = createRemoteJWKSet(new URL("/.well-known/jwks.json", API_URL));

export interface Session {
  userId: string;
  sessionId: string;
  roles: string[];
  scopes: string[];
  lang: string;
  expiresAt: number; // unix seconds
}

function strings(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((v): v is string => typeof v === "string") : [];
}

// verifyAccessToken returns the session for a valid token, or null. The UI
// uses it only to decide what to show; core-api enforces access itself.
export async function verifyAccessToken(token: string): Promise<Session | null> {
  try {
    const { payload } = await jwtVerify(token, jwks, {
      issuer: JWT_ISSUER,
      audience: JWT_AUDIENCE,
      algorithms: ["EdDSA"],
    });
    if (typeof payload.sub !== "string" || typeof payload.exp !== "number") return null;
    return {
      userId: payload.sub,
      sessionId: typeof payload.sid === "string" ? payload.sid : "",
      roles: strings(payload.roles),
      scopes: strings(payload.scopes),
      lang: typeof payload.lang === "string" ? payload.lang : "en",
      expiresAt: payload.exp,
    };
  } catch {
    return null;
  }
}
