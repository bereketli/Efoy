import type { NextConfig } from "next";

// The browser calls /api/efoy/*; Next.js proxies it to core-api, so the
// console needs no CORS setup. Rewrites are resolved at build time.
const apiUrl = process.env.EFOY_API_URL ?? "http://localhost:8080";

const nextConfig: NextConfig = {
  output: "standalone",
  async rewrites() {
    return [{ source: "/api/efoy/:path*", destination: `${apiUrl}/:path*` }];
  },
};

export default nextConfig;
