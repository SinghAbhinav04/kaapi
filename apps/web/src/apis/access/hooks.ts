'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { authKeys } from '@/apis/auth';

import { accessApi } from './access.api';
import { accessKeys } from './const';
import { accessQueries } from './queries';
import type { CreateMemberRequest, RoleRequest, UpdateMemberRequest } from './types';

export const useAccessOverview = () => useQuery(accessQueries.overview());
export const useMembers = () => useQuery(accessQueries.members());

/** Any change can alter role counts, people's roles and (for the admin) their own permissions. */
function useAccessMutation<V, R>(fn: (vars: V) => Promise<R>) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: fn,
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: accessKeys.all });
      void queryClient.invalidateQueries({ queryKey: authKeys.me() });
    },
  });
}

export const useCreateRole = () => useAccessMutation((body: RoleRequest) => accessApi.createRole(body));
export const useUpdateRole = () => useAccessMutation(({ id, body }: { id: string; body: RoleRequest }) => accessApi.updateRole(id, body));
export const useDeleteRole = () => useAccessMutation((id: string) => accessApi.deleteRole(id));
export const useCreateMember = () => useAccessMutation((body: CreateMemberRequest) => accessApi.createMember(body));
export const useUpdateMember = () =>
  useAccessMutation(({ id, body }: { id: string; body: UpdateMemberRequest }) => accessApi.updateMember(id, body));
