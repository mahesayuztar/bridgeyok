import type { NextConfig } from "next";

if (process.env.VERCEL === "1") {
  const publicApiUrl = process.env.NEXT_PUBLIC_API_BASE_URL;
  if (!publicApiUrl) throw new Error("NEXT_PUBLIC_API_BASE_URL is required on Vercel");
  const apiUrl = new URL(publicApiUrl);
  if (apiUrl.protocol !== "https:" || apiUrl.username || apiUrl.password || apiUrl.pathname !== "/" || apiUrl.search || apiUrl.hash) {
    throw new Error("NEXT_PUBLIC_API_BASE_URL must be an HTTPS origin on Vercel");
  }
  if (process.env.API_BASE_URL !== undefined && new URL(process.env.API_BASE_URL).href !== apiUrl.href) {
    throw new Error("API_BASE_URL and NEXT_PUBLIC_API_BASE_URL must target the same API on Vercel");
  }
}

const nextConfig: NextConfig = {
  poweredByHeader: false,
  ...(process.env.NEXT_DIST_DIR === ".next-e2e" ? { devIndicators: false as const } : {}),
  distDir: process.env.NEXT_DIST_DIR ?? ".next",
  // output: "standalone",
  allowedDevOrigins: ["192.168.1.6"],
  async headers() {
    return [
      {
        source: "/:path*",
        headers: [
          { key: "X-Content-Type-Options", value: "nosniff" },
          { key: "X-Frame-Options", value: "DENY" },
          { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
          { key: "Permissions-Policy", value: "camera=(), microphone=(), geolocation=(), browsing-topics=()" }
        ]
      }
    ];
  }
};

export default nextConfig;
