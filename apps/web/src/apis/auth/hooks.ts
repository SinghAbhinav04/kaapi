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

/** Staff sign-in step 1: checks the password and emails a code. */
export const useStaffLogin = () =>
  useMutation({ mutationFn: ({ email, password }: { email: string; password: string }) => authApi.staffLogin(email, password) });

/** Staff sign-in step 2: the emailed code. */
export function useStaffVerify() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ email, code }: { email: string; code: string }) => authApi.staffVerify(email, code),
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
