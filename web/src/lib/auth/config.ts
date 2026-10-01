// Server-side auth settings. Read at runtime, so one image serves every
// environment.

export const API_URL = process.env.EFOY_API_URL ?? "http://localhost:8080";

// Must match core-api's auth.issuer and auth.audience.
export const JWT_ISSUER = process.env.EFOY_JWT_ISSUER ?? "efoy";
export const JWT_AUDIENCE = process.env.EFOY_JWT_AUDIENCE ?? "efoy-api";

export const ACCESS_COOKIE = "efoy_at";
export const REFRESH_COOKIE = "efoy_rt";

// Portal sessions end 12 h after login (core-api auth.staff_session_ttl).
export const STAFF_SESSION_SECONDS = 12 * 60 * 60;

export const SECURE_COOKIES = process.env.NODE_ENV === "production";
