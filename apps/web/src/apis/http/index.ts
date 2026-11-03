import { createHttpClient } from './client';
import { BROWSER_BASE_URL, REFRESH_PATH, SERVER_BASE_URL } from './const';
import type { HttpClient } from './interface';

const isServer = typeof window === 'undefined';

/** Shared by every tab of this site, so only one renews the session at a time. */
const REFRESH_LOCK = 'kaapi-auth-refresh';

/**
 * Renews the session cookies. Tabs share cookies but not memory: two tabs refreshing at once
 * would send the same refresh token twice, which the API treats as theft and answers by
 * revoking every session. Under the lock the second tab sends the cookie the first one just
 * received, so the rotation stays a clean chain.
 */
async function refreshSession(): Promise<boolean> {
  const renew = async () => (await fetch(`${BROWSER_BASE_URL}${REFRESH_PATH}`, { method: 'POST', credentials: 'include' })).ok;
  return navigator.locks ? navigator.locks.request(REFRESH_LOCK, renew) : renew();
}

/**
 * The app's single HTTP client. In the browser it goes through the /api proxy with cookies;
 * on the server (public pages only) it calls the API directly.
 */
export const http: HttpClient = createHttpClient({
  baseUrl: isServer ? SERVER_BASE_URL : BROWSER_BASE_URL,
  refreshSession: isServer ? undefined : refreshSession,
});

export { createHttpClient } from './client';
export { HEADER } from './const';
export { ApiError, isApiError, isRetryable } from './errors';
export type { HttpClient } from './interface';
export type { Query, RequestOptions } from './types';
