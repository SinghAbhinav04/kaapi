import type { ErrorCode } from '@/apis/generated/schema';

import type { ApiErrorPayload } from './types';

/** Codes the client produces itself, for failures that never reached the API. */
export type ClientErrorCode = 'NETWORK' | 'TIMEOUT' | 'ABORTED' | 'BAD_RESPONSE';

/** Every failed API call rejects with this, carrying the API's stable error code. */
export class ApiError extends Error {
  override readonly name = 'ApiError';

  constructor(
    readonly status: number,
    readonly code: ErrorCode | ClientErrorCode,
    message: string,
    readonly details: Record<string, unknown> = {},
    readonly requestId?: string,
  ) {
    super(message);
  }

  static fromPayload(status: number, payload: ApiErrorPayload): ApiError {
    return new ApiError(status, payload.code, payload.message, payload.details ?? {}, payload.request_id);
  }

  /** Per-field messages from a VALIDATION_FAILED response. */
  get fieldErrors(): Record<string, string> {
    const fields = this.details.fields;
    return fields && typeof fields === 'object' ? (fields as Record<string, string>) : {};
  }

  get isNetwork(): boolean {
    return this.code === 'NETWORK' || this.code === 'TIMEOUT';
  }
}

export function isApiError(error: unknown): error is ApiError {
  return error instanceof ApiError;
}

/** True when retrying could help: network trouble or a server-side hiccup. */
export function isRetryable(error: unknown): boolean {
  if (!isApiError(error)) return false;
  return error.isNetwork || error.status >= 500 || error.code === 'RATE_LIMITED';
}
