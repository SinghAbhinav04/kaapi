import type { DashboardResponse, OutboxResponse } from './types';

/** Staff oversight: what the system has "sent", and the success-measure numbers. */
export interface OperationsApi {
  outbox(limit: number, signal?: AbortSignal): Promise<OutboxResponse>;
  dashboard(signal?: AbortSignal): Promise<DashboardResponse>;
}
