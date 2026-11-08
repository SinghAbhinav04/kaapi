export const NOTIFICATIONS_PATHS = {
  list: '/v1/staff/notifications',
  readAll: '/v1/staff/notifications/read',
  read: (id: string) => `/v1/staff/notifications/${encodeURIComponent(id)}/read`,
} as const;

export const notificationKeys = {
  all: ['notifications'] as const,
  list: () => [...notificationKeys.all, 'list'] as const,
};

/** The change stream refreshes alerts the moment one arrives; this is only the safety net. */
export const NOTIFICATIONS_REFRESH_MS = 120_000;
