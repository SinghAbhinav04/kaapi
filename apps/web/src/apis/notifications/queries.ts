import { queryOptions } from '@tanstack/react-query';

import { NOTIFICATIONS_REFRESH_MS, notificationKeys } from './const';
import { notificationsApi } from './notifications.api';

export const notificationQueries = {
  // New alerts arrive through the change stream (apis/changes); the interval is a safety net.
  list: () =>
    queryOptions({
      queryKey: notificationKeys.list(),
      queryFn: ({ signal }) => notificationsApi.list(signal),
      refetchInterval: NOTIFICATIONS_REFRESH_MS,
    }),
};
