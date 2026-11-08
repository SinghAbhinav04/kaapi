import type { MetadataRoute } from 'next';

/** Guests' pages are public; the staff console and the API proxy are not for crawlers. */
export default function robots(): MetadataRoute.Robots {
  return { rules: { userAgent: '*', allow: '/', disallow: ['/staff', '/api'] } };
}
