import type { NextConfig } from "next";

// The browser never talks to core-api directly: /api/efoy/* and /api/auth/*
// are route handlers that attach the session from httpOnly cookies.
const nextConfig: NextConfig = {
  output: "standalone",
};

export default nextConfig;
