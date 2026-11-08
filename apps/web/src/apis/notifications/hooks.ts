'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { notificationKeys } from './const';
import { notificationsApi } from './notifications.api';
import { notificationQueries } from './queries';

export const useNotifications = (enabled = true) => useQuery({ ...notificationQueries.list(), enabled });

export function useMarkNotificationRead() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => notificationsApi.markRead(id),
    onSettled: () => queryClient.invalidateQueries({ queryKey: notificationKeys.all }),
  });
}

export function useMarkAllNotificationsRead() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => notificationsApi.markAllRead(),
    onSettled: () => queryClient.invalidateQueries({ queryKey: notificationKeys.all }),
  });
}
