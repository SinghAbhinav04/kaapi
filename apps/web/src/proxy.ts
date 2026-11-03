import { NextResponse, type NextRequest } from 'next/server';

import { CURRENCY_COOKIE, CURRENCY_COOKIE_MAX_AGE_S, currencyForCountry } from '@/features/currency/geo';

/** Vercel's edge reports the visitor's country on every request. */
const COUNTRY_HEADER = 'x-vercel-ip-country';

/**
 * On a guest's first visit, pick the currency for the "≈" estimates from their country, so
 * the right one is in the cookie before the page hydrates. Without the header (local runs,
 * other hosts) nothing is set and the page guesses from the browser's time zone instead.
 */
export function proxy(request: NextRequest) {
  const response = NextResponse.next();
  const country = request.headers.get(COUNTRY_HEADER);
  if (country && !request.cookies.has(CURRENCY_COOKIE)) {
    response.cookies.set(CURRENCY_COOKIE, currencyForCountry(country), {
      path: '/',
      maxAge: CURRENCY_COOKIE_MAX_AGE_S,
      sameSite: 'lax',
    });
  }
  return response;
}

// Guest pages only: not the API proxy, Next's own files, images or the staff console.
export const config = {
  matcher: ['/((?!api/|_next/|images/|staff|favicon.ico).*)'],
};
