'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { bookingKeys } from '@/apis/booking/const';

import { paymentKeys } from './const';
import { paymentsApi } from './payments.api';
import { paymentQueries } from './queries';
import type { PaymentOutcome } from './types';

export const usePaymentPage = (orderId: string) => useQuery(paymentQueries.page(orderId));

export function useSimulatePayment(orderId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (outcome: PaymentOutcome) => paymentsApi.simulate(orderId, outcome),
    onSettled: async () => {
      await queryClient.invalidateQueries({ queryKey: paymentKeys.page(orderId) });
      await queryClient.invalidateQueries({ queryKey: bookingKeys.all });
    },
  });
}
