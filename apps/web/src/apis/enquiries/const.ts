import type { InboxParams } from './types';

const staff = (ref: string) => `/v1/staff/enquiries/${encodeURIComponent(ref)}`;

export const ENQUIRY_PATHS = {
  create: '/v1/enquiries',
  inbound: '/v1/staff/inbound',
  list: '/v1/staff/enquiries',
  detail: staff,
  route: (ref: string) => `${staff(ref)}/route`,
  fields: (ref: string) => `${staff(ref)}/fields`,
  draft: (ref: string) => `${staff(ref)}/draft`,
  send: (ref: string) => `${staff(ref)}/send`,
  status: (ref: string) => `${staff(ref)}/status`,
  retryAi: (ref: string) => `${staff(ref)}/retry-ai`,
  stream: '/v1/staff/inbox/stream',
  syncMail: '/v1/staff/integrations/gmail/sync',
} as const;

export const enquiryKeys = {
  all: ['enquiries'] as const,
  lists: () => [...enquiryKeys.all, 'list'] as const,
  list: (params: InboxParams) => [...enquiryKeys.lists(), params] as const,
  detail: (ref: string) => [...enquiryKeys.all, 'detail', ref] as const,
};

/**
 * The inbox refetches in the background so new messages and AI results appear without
 * a reload; the list on screen never blanks while it does.
 */
export const INBOX_REFRESH_MS = 15_000;
export const INBOX_STALE_MS = 10_000;
/** While the AI is still reading an enquiry, its detail checks back more often. */
export const AI_PENDING_POLL_MS = 4_000;

/** After the live stream drops (e.g. the session expired), wait this long before reopening it. */
export const STREAM_RETRY_MS = 5_000;
