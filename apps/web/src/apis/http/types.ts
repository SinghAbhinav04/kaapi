import type { ErrorCode } from '@/apis/generated/schema';

export type HttpMethod = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';

export type QueryValue = string | number | boolean | null | undefined;
export type Query = Record<string, QueryValue>;

export interface RequestOptions {
  query?: Query;
  body?: unknown;
  headers?: Record<string, string>;
  signal?: AbortSignal;
  /** Return the raw text body instead of parsing JSON (CSV exports). */
  responseType?: 'json' | 'text';
}

export interface HttpClientConfig {
  baseUrl: string;
  /** Called once per burst of 401s; resolve true if the session was renewed. */
  refreshSession?: () => Promise<boolean>;
  fetchImpl?: typeof fetch;
  timeoutMs?: number;
}

/** The API's error envelope, as every endpoint returns it. */
export interface ApiErrorPayload {
  code: ErrorCode;
  message: string;
  details?: Record<string, unknown>;
  request_id?: string;
}
