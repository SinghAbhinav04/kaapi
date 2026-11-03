import type {
  ComposeRequest,
  DashboardResponse,
  GmailConnectResponse,
  GmailStatusResponse,
  GmailSyncResponse,
  OutboxMessageResponse,
  OutboxResponse,
} from './types';

/** Staff oversight: what the system has sent, the success-measure numbers, and the connected mailbox. */
export interface OperationsApi {
  outbox(limit: number, signal?: AbortSignal): Promise<OutboxResponse>;
  dashboard(signal?: AbortSignal): Promise<DashboardResponse>;
  compose(body: ComposeRequest): Promise<OutboxMessageResponse>;
  gmailStatus(signal?: AbortSignal): Promise<GmailStatusResponse>;
  gmailConnect(): Promise<GmailConnectResponse>;
  gmailSync(): Promise<GmailSyncResponse>;
  gmailDisconnect(): Promise<GmailStatusResponse>;
}
