import { BROWSER_BASE_URL, REFRESH_PATH } from './const';

/** Shared by every tab of this site, so only one renews the session at a time. */
const REFRESH_LOCK = 'kaapi-auth-refresh';

/**
 * Renews the session cookies. Tabs share cookies but not memory: two tabs refreshing at once
 * would send the same refresh token twice, which the API treats as theft and answers by
 * revoking every session. Under the lock the second tab sends the cookie the first one just
 * received, so the rotation stays a clean chain.
 */
export async function refreshSession(): Promise<boolean> {
  const renew = async () => (await fetch(`${BROWSER_BASE_URL}${REFRESH_PATH}`, { method: 'POST', credentials: 'include' })).ok;
  return navigator.locks ? navigator.locks.request(REFRESH_LOCK, renew) : renew();
}
