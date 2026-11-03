import { QueryClient, defaultShouldDehydrateQuery, isServer } from '@tanstack/react-query';

import { isRetryable } from '@/apis/http';

import { QUERY_DEFAULTS } from './const';

export function makeQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: QUERY_DEFAULTS.staleTime,
        gcTime: QUERY_DEFAULTS.gcTime,
        // Retry network trouble and server hiccups, never a 4xx: it would fail the same way.
        retry: (count, error) => count < QUERY_DEFAULTS.maxRetries && isRetryable(error),
        refetchOnWindowFocus: true,
      },
      mutations: { retry: false },
      dehydrate: {
        // Also hand pending server prefetches to the client, so it doesn't fetch twice.
        shouldDehydrateQuery: (query) => defaultShouldDehydrateQuery(query) || query.state.status === 'pending',
      },
    },
  });
}

let browserClient: QueryClient | undefined;

/** One client per browser tab; a fresh one per server request, so users never share a cache. */
export function getQueryClient(): QueryClient {
  if (isServer) return makeQueryClient();
  browserClient ??= makeQueryClient();
  return browserClient;
}
