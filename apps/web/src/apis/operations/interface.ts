import type { ComposeRequest, DashboardResponse, GmailStatusResponse, OutboxMessageResponse, OutboxResponse } from './types';

/** Staff oversight: what the system has sent, the success-measure numbers, and the mailbox's health. */
export interface OperationsApi {
  outbox(limit: number, signal?: AbortSignal): Promise<OutboxResponse>;
  dashboard(signal?: AbortSignal): Promise<DashboardResponse>;
  compose(body: ComposeRequest): Promise<OutboxMessageResponse>;
  gmailStatus(signal?: AbortSignal): Promise<GmailStatusResponse>;
}
