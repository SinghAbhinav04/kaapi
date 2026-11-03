import { NextResponse, type NextRequest } from 'next/server';

// Google's OAuth client for this project lists the site root as its redirect URI, so the
// Gmail consent screen comes back to "/?code=…&state=…". Hand that straight to the API's
// callback, which verifies the signed state and stores the connection.
const GMAIL_CALLBACK = '/api/v1/integrations/gmail/callback';
const GMAIL_SCOPE_HINT = 'gmail';

export function proxy(request: NextRequest) {
  const { searchParams } = request.nextUrl;
  const granted = searchParams.has('code') && (searchParams.get('scope') ?? '').includes(GMAIL_SCOPE_HINT);
  const declined = searchParams.has('error');
  if (!searchParams.has('state') || !(granted || declined)) return NextResponse.next();
  const target = request.nextUrl.clone();
  target.pathname = GMAIL_CALLBACK;
  return NextResponse.redirect(target);
}

export const config = { matcher: '/' };
