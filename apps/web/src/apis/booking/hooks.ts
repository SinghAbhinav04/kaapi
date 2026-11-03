'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { catalogKeys } from '@/apis/catalog/const';

import { bookingApi } from './booking.api';
import { bookingKeys } from './const';
import { bookingQueries } from './queries';
import type { CreateBookingRequest, CreateHoldRequest, StaffBookingsParams } from './types';

export const useBooking = (ref: string, holdToken?: string) => useQuery(bookingQueries.detail(ref, holdToken));

export const useMyBookings = (enabled = true) => useQuery({ ...bookingQueries.mine(), enabled });

export const useStaffBookings = (params: StaffBookingsParams) => useQuery(bookingQueries.staff(params));

/** A hold changes what's left, so availability is refetched afterwards. */
export function useCreateHold() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: CreateHoldRequest) => bookingApi.createHold(body),
    onSettled: () => queryClient.invalidateQueries({ queryKey: catalogKeys.all }),
  });
}

export function useReleaseHold() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ holdId, token }: { holdId: string; token: string }) => bookingApi.releaseHold(holdId, token),
    onSettled: () => queryClient.invalidateQueries({ queryKey: catalogKeys.all }),
  });
}

export function useCreateBooking() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ body, token }: { body: CreateBookingRequest; token: string }) => bookingApi.createBooking(body, token),
    onSuccess: (data) => queryClient.setQueryData(bookingKeys.detail(data.booking.ref), data),
  });
}

export const useExportDay = () => useMutation({ mutationFn: (date: string) => bookingApi.exportDay(date) });
