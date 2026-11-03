import { createHttpClient } from './client';
import { BROWSER_BASE_URL, REFRESH_PATH, SERVER_BASE_URL } from './const';
import type { HttpClient } from './interface';

const isServer = typeof window === 'undefined';

/**
 * The app's single HTTP client. In the browser it goes through the /api proxy with cookies;
 * on the server (public pages only) it calls the API directly.
 */
export const http: HttpClient = createHttpClient({
  baseUrl: isServer ? SERVER_BASE_URL : BROWSER_BASE_URL,
  refreshSession: isServer
    ? undefined
    : async () => {
        const res = await fetch(`${BROWSER_BASE_URL}${REFRESH_PATH}`, { method: 'POST', credentials: 'include' });
        return res.ok;
      },
});

export { createHttpClient } from './client';
export { HEADER } from './const';
export { ApiError, isApiError, isRetryable } from './errors';
export type { HttpClient } from './interface';
export type { Query, RequestOptions } from './types';

export { BROWSER_BASE_URL } from './const';
