import type { NextConfig } from "next";

const isDev = process.env.NODE_ENV === "development";

const nextConfig: NextConfig = {
  reactStrictMode: true,
  poweredByHeader: false,
  // next dev would otherwise write AGENTS.md into the repo root.
  agentRules: false,
  turbopack: { root: import.meta.dirname },
  async rewrites() {
    // In production the Go handlers are served on the same origin.
    if (!isDev) return [];
    return [{ source: "/api/:path*", destination: "http://127.0.0.1:8080/api/:path*" }];
  },
};

export default nextConfig;
