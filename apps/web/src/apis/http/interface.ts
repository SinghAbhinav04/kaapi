import type { RequestOptions } from './types';

/**
 * Everything the API modules need from HTTP. Modules depend on this interface, not on
 * fetch, so tests or another transport (e.g. the Capacitor app's native bridge) can
 * stand in without touching them.
 */
export interface HttpClient {
  get<T>(path: string, options?: RequestOptions): Promise<T>;
  post<T>(path: string, body?: unknown, options?: RequestOptions): Promise<T>;
  put<T>(path: string, body?: unknown, options?: RequestOptions): Promise<T>;
  patch<T>(path: string, body?: unknown, options?: RequestOptions): Promise<T>;
  delete<T = void>(path: string, options?: RequestOptions): Promise<T>;
}
