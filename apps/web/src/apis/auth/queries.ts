import { queryOptions } from '@tanstack/react-query';

import { isApiError } from '@/apis/http';

import { authApi } from './auth.api';
import { AUTH_STALE_MS, authKeys } from './const';

export const authQueries = {
  // A 401 means "signed out", not an error to retry.
  me: () =>
    queryOptions({
      queryKey: authKeys.me(),
      queryFn: async ({ signal }) => {
        try {
          return (await authApi.me(signal)).user;
        } catch (error) {
          if (isApiError(error) && error.status === 401) return null;
          throw error;
        }
      },
      staleTime: AUTH_STALE_MS,
      retry: false,
    }),
};
