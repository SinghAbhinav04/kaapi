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
  poweredByHeader: false,
  agentRules: false,
  images: {
    formats: ['image/avif', 'image/webp'],
    // Next 16 only serves listed qualities; 60 for large photography, 75 for the rest.
    qualities: [60, 75],
    deviceSizes: [390, 640, 828, 1080, 1440, 1920, 2400],
    minimumCacheTTL: 60 * 60 * 24 * 30,
  },
  async rewrites() {
    return [{ source: '/api/:path*', destination: `${apiOrigin}/:path*` }];
  },
  async headers() {
    return [{ source: '/:path*', headers: securityHeaders }];
  },
};

export default config;
