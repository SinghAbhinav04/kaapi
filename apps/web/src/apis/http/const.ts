/** Where the browser reaches the API: same origin, proxied by next.config.ts. */
export const BROWSER_BASE_URL = '/api';

/** Where server components reach the API directly. */
export const SERVER_BASE_URL = process.env.API_ORIGIN ?? 'http://localhost:8080';

export const REQUEST_TIMEOUT_MS = 20_000;

export const HEADER = {
  holdToken: 'X-Hold-Token',
  requestId: 'X-Request-Id',
  client: 'X-Client',
} as const;

/** The one call that may run when a request comes back 401, to renew the session. */
export const REFRESH_PATH = '/v1/auth/refresh';

/** Auth calls that must never trigger a refresh-and-retry loop. */
export const NO_REFRESH_PATHS: readonly string[] = [
  REFRESH_PATH,
  '/v1/auth/staff/login',
  '/v1/auth/staff/verify',
  '/v1/auth/otp/request',
  '/v1/auth/otp/verify',
  '/v1/auth/logout',
];

/** How long a closed event stream waits before reconnecting. */
export const STREAM_RETRY_MS = 5_000;
