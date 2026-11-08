import { http, type HttpClient } from '@/apis/http';

import { NOTIFICATIONS_PATHS } from './const';
import type { NotificationsApi } from './interface';

export function createNotificationsApi(client: HttpClient): NotificationsApi {
  return {
    list: (signal) => client.get(NOTIFICATIONS_PATHS.list, { signal }),
    markRead: (id) => client.post(NOTIFICATIONS_PATHS.read(id)),
    markAllRead: () => client.post(NOTIFICATIONS_PATHS.readAll),
  };
}

export const notificationsApi = createNotificationsApi(http);
