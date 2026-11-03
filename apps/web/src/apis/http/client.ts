import { NO_REFRESH_PATHS, REQUEST_TIMEOUT_MS } from './const';
import { ApiError } from './errors';
import type { HttpClient } from './interface';
import type { ApiErrorPayload, HttpClientConfig, HttpMethod, Query, RequestOptions } from './types';

/**
 * fetch-based HttpClient. It sends cookies, times out slow calls (the resort's guests are
 * often on weak mobile signal), turns error envelopes into ApiError, and on a 401 renews
 * the session once and retries, sharing one refresh between concurrent calls.
 */
export function createHttpClient(config: HttpClientConfig): HttpClient {
  const fetchImpl = config.fetchImpl ?? fetch;
  const timeoutMs = config.timeoutMs ?? REQUEST_TIMEOUT_MS;
  let refreshing: Promise<boolean> | null = null;

  const renew = (): Promise<boolean> => {
    if (!config.refreshSession) return Promise.resolve(false);
    refreshing ??= config.refreshSession().finally(() => {
      refreshing = null;
    });
    return refreshing;
  };

  async function send<T>(method: HttpMethod, path: string, options: RequestOptions = {}, retried = false): Promise<T> {
    const timeout = AbortSignal.timeout(timeoutMs);
    const signal = options.signal ? AbortSignal.any([options.signal, timeout]) : timeout;
    const headers: Record<string, string> = { Accept: 'application/json', ...options.headers };
    let body: BodyInit | undefined;
    if (options.body !== undefined) {
      headers['Content-Type'] = 'application/json';
      body = JSON.stringify(options.body);
    }

    let response: Response;
    try {
      response = await fetchImpl(buildUrl(config.baseUrl, path, options.query), {
        method,
        headers,
        body,
        signal,
        credentials: 'include',
        cache: 'no-store',
      });
    } catch (error) {
      if (options.signal?.aborted) throw new ApiError(0, 'ABORTED', 'The request was cancelled.');
      if (timeout.aborted) throw new ApiError(0, 'TIMEOUT', 'This is taking too long. Check your connection and try again.');
      throw new ApiError(0, 'NETWORK', "We couldn't reach the server. Check your connection and try again.", {
        cause: String(error),
      });
    }

    if (response.status === 401 && !retried && !NO_REFRESH_PATHS.includes(path) && (await renew())) {
      return send<T>(method, path, options, true);
    }
    if (!response.ok) throw await toApiError(response);
    if (response.status === 204) return undefined as T;
    if (options.responseType === 'text') return (await response.text()) as T;
    try {
      return (await response.json()) as T;
    } catch {
      throw new ApiError(response.status, 'BAD_RESPONSE', 'The server sent a response we could not read.');
    }
  }

  return {
    get: (path, options) => send('GET', path, options),
    post: (path, body, options) => send('POST', path, { ...options, body }),
    put: (path, body, options) => send('PUT', path, { ...options, body }),
    patch: (path, body, options) => send('PATCH', path, { ...options, body }),
    delete: (path, options) => send('DELETE', path, options),
  };
}

function buildUrl(baseUrl: string, path: string, query?: Query): string {
  const url = `${baseUrl.replace(/\/$/, '')}${path}`;
  if (!query) return url;
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined && value !== null && value !== '') params.set(key, String(value));
  }
  const qs = params.toString();
  return qs ? `${url}?${qs}` : url;
}

async function toApiError(response: Response): Promise<ApiError> {
  try {
    const payload = (await response.json()) as { error?: ApiErrorPayload };
    if (payload.error?.code) return ApiError.fromPayload(response.status, payload.error);
  } catch {
    // not our envelope (e.g. a proxy error page); fall through
  }
  return new ApiError(response.status, response.status >= 500 ? 'INTERNAL' : 'BAD_REQUEST', 'Something went wrong. Please try again.');
}
