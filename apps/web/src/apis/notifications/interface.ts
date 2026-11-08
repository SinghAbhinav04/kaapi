import type { NotificationsResponse } from './types';

/** The console's alerts: complaints, enquiries to sort, refunds needed, for the caller's team. */
export interface NotificationsApi {
  list(signal?: AbortSignal): Promise<NotificationsResponse>;
  markRead(id: string): Promise<void>;
  markAllRead(): Promise<void>;
}
