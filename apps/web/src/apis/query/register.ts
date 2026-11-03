import type { ApiError } from '@/apis/http';

/**
 * Every query and mutation goes through the HttpClient, which only ever rejects with
 * ApiError, so components can read `error.code`, `error.requestId` and field errors.
 */
declare module '@tanstack/react-query' {
  interface Register {
    defaultError: ApiError;
  }
}
