import { NextResponse, type NextRequest } from 'next/server';

import { CURRENCY_COOKIE, CURRENCY_COOKIE_MAX_AGE_S, currencyForCountry } from '@/features/currency/geo';

// Google's OAuth client for this project lists the site root as its redirect URI, so the
// Gmail consent screen comes back to "/?code=…&state=…". Hand that straight to the API's
// callback, which verifies the signed state and stores the connection.
const GMAIL_CALLBACK = '/api/v1/integrations/gmail/callback';
const GMAIL_SCOPE_HINT = 'gmail';

/** Vercel's edge reports the visitor's country on every request. */
const COUNTRY_HEADER = 'x-vercel-ip-country';

export function proxy(request: NextRequest) {
  return gmailReturn(request) ?? withCurrency(request);
}

function gmailReturn(request: NextRequest): NextResponse | null {
  const { pathname, searchParams } = request.nextUrl;
  if (pathname !== '/') return null;
  const granted = searchParams.has('code') && (searchParams.get('scope') ?? '').includes(GMAIL_SCOPE_HINT);
  const declined = searchParams.has('error');
  if (!searchParams.has('state') || !(granted || declined)) return null;
  const target = request.nextUrl.clone();
  target.pathname = GMAIL_CALLBACK;
  return NextResponse.redirect(target);
}

/**
 * On a guest's first visit, pick the currency for the "≈" estimates from their country, so
 * the right one is in the cookie before the page hydrates. Without the header (local runs,
 * other hosts) nothing is set and the page guesses from the browser's time zone instead.
 */
function withCurrency(request: NextRequest): NextResponse {
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
