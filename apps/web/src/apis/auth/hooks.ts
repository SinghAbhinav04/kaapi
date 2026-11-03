'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { authApi } from './auth.api';
import { authKeys } from './const';
import { authQueries } from './queries';
import type { Permission, User } from './types';

/** The signed-in user, or null. */
export const useMe = () => useQuery(authQueries.me());

export function useCan(permission: Permission): boolean {
  const { data } = useMe();
  return hasPermission(data ?? null, permission);
}

export function hasPermission(user: User | null, permission: Permission): boolean {
  return user?.permissions.includes(permission) ?? false;
}

export function useStaffLogin() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ email, password }: { email: string; password: string }) => authApi.staffLogin(email, password),
    onSuccess: (data) => queryClient.setQueryData(authKeys.me(), data.user),
  });
}

export const useRequestOtp = () => useMutation({ mutationFn: (email: string) => authApi.requestOtp(email) });

export function useVerifyOtp() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ email, code }: { email: string; code: string }) => authApi.verifyOtp(email, code),
    onSuccess: (data) => queryClient.setQueryData(authKeys.me(), data.user),
  });
}

/** Signing out drops every cached response, so nothing private lingers in memory. */
export function useLogout() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => authApi.logout(),
    onSettled: () => {
      queryClient.clear();
      queryClient.setQueryData(authKeys.me(), null);
    },
  });
}
