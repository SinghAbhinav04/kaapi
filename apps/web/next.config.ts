import type { NextConfig } from 'next';

// The browser only ever talks to this site: /api/* is proxied to the Go API, so cookies
// stay first-party and no CORS is needed. Server components call API_ORIGIN directly.
const apiOrigin = process.env.API_ORIGIN ?? 'http://localhost:8080';

const securityHeaders = [
  { key: 'X-Content-Type-Options', value: 'nosniff' },
  { key: 'Referrer-Policy', value: 'strict-origin-when-cross-origin' },
  { key: 'X-Frame-Options', value: 'DENY' },
  { key: 'Permissions-Policy', value: 'camera=(), microphone=(), geolocation=()' },
];

const config: NextConfig = {
  reactCompiler: true,
  // Keep visited and prefetched pages in the browser for 5 minutes, so moving between staff
  // pages reuses them instead of fetching the page again. The data inside is TanStack Query's
  // job (fresh until the change stream says otherwise), so the pages themselves rarely change.
  experimental: { staleTimes: { dynamic: 300, static: 300 } },
  poweredByHeader: false,
  agentRules: false,
  images: {
    // Sizes are pre-built AVIF files (scripts/build-images.py); the loader picks one per
    // device instead of asking Vercel's optimizer, which passed AVIF through at full size.
    loader: 'custom',
    loaderFile: './src/shared/lib/image-loader.ts',
    // Next 16 only accepts listed qualities; 60 for large photography, 75 for the rest.
    qualities: [60, 75],
    deviceSizes: [390, 640, 828, 1080, 1440, 1920, 2400],
  },
  async rewrites() {
    return [{ source: '/api/:path*', destination: `${apiOrigin}/:path*` }];
  },
  async headers() {
    return [
      { source: '/:path*', headers: securityHeaders },
      // Photos are rebuilt rarely; let browsers and the CDN keep them for a month.
      { source: '/images/:path*', headers: [{ key: 'Cache-Control', value: 'public, max-age=2592000, stale-while-revalidate=86400' }] },
    ];
  },
};

export default config;
