export type EnquiryKind = 'GENERAL' | 'EVENT';

export interface EnquiryForm {
  kind: EnquiryKind;
  name: string;
  email: string;
  phone: string;
  message: string;
  eventType: string;
  /** YYYY-MM */
  month: string;
  guests: string;
  nights: string;
  /** Rupees, as typed. */
  budget: string;
}
