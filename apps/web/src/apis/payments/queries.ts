import { queryOptions } from '@tanstack/react-query';

import { paymentKeys } from './const';
import { paymentsApi } from './payments.api';

export const paymentQueries = {
  page: (orderId: string) =>
    queryOptions({
      queryKey: paymentKeys.page(orderId),
      queryFn: ({ signal }) => paymentsApi.getPage(orderId, signal),
      staleTime: 0,
    }),
};
