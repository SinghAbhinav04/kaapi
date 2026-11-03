import { keepPreviousData, queryOptions } from '@tanstack/react-query';

import { AI_PENDING_POLL_MS, INBOX_REFRESH_MS, INBOX_STALE_MS, enquiryKeys } from './const';
import { enquiriesApi } from './enquiries.api';
import type { InboxParams } from './types';

export const enquiryQueries = {
  // Switching lanes or typing a search keeps the previous list on screen until the new one lands.
  list: (params: InboxParams) =>
    queryOptions({
      queryKey: enquiryKeys.list(params),
      queryFn: ({ signal }) => enquiriesApi.list(params, signal),
      staleTime: INBOX_STALE_MS,
      refetchInterval: INBOX_REFRESH_MS,
      placeholderData: keepPreviousData,
    }),
  detail: (ref: string) =>
    queryOptions({
      queryKey: enquiryKeys.detail(ref),
      queryFn: ({ signal }) => enquiriesApi.get(ref, signal),
      staleTime: INBOX_STALE_MS,
      refetchInterval: (query) => (query.state.data?.enquiry.ai_status === 'PENDING' ? AI_PENDING_POLL_MS : false),
    }),
};
