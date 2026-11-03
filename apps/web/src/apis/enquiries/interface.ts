import type {
  CorrectEnquiryRequest,
  CreateEnquiryRequest,
  EnquiriesResponse,
  EnquiryCreatedResponse,
  EnquiryResponse,
  EnquiryStatus,
  InboundMessageRequest,
  InboxParams,
  RouteRequest,
} from './types';

/** Guest enquiries in, staff inbox out. Replies are only ever sent by `send`. */
export interface EnquiriesApi {
  create(body: CreateEnquiryRequest): Promise<EnquiryCreatedResponse>;
  /** Simulated email or WhatsApp message, entering the same pipeline a webhook would feed. */
  inbound(body: InboundMessageRequest): Promise<EnquiryCreatedResponse>;
  list(params: InboxParams, signal?: AbortSignal): Promise<EnquiriesResponse>;
  get(ref: string, signal?: AbortSignal): Promise<EnquiryResponse>;
  route(ref: string, body: RouteRequest): Promise<EnquiryResponse>;
  correct(ref: string, body: CorrectEnquiryRequest): Promise<EnquiryResponse>;
  editDraft(ref: string, body: string): Promise<EnquiryResponse>;
  send(ref: string): Promise<EnquiryResponse>;
  setStatus(ref: string, status: EnquiryStatus): Promise<EnquiryResponse>;
  retryAi(ref: string): Promise<EnquiryResponse>;
}
