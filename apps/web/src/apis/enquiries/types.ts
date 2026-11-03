import type { Category, EnquiryStatus, Lane } from '@/apis/generated/schema';

export type {
  AiStatus,
  InboxCountsResponse,
  Category,
  Channel,
  CorrectEnquiryRequest,
  CreateEnquiryRequest,
  Draft,
  EnquiriesResponse,
  Enquiry,
  EnquiryCreatedResponse,
  EnquiryEvent,
  EnquiryResponse,
  EnquiryStatus,
  EnquirySummary,
  EventDetails,
  Extracted,
  InboundMessageRequest,
  Lane,
} from '@/apis/generated/schema';

export interface InboxParams {
  lane?: Lane;
  status?: EnquiryStatus;
  q?: string;
  limit?: number;
}

export interface RouteRequest {
  lane: Lane;
  category?: Category;
}
