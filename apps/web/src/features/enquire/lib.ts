import type { CreateEnquiryRequest } from '@/apis/enquiries';
import { rupeesToPaise } from '@/shared/lib/money';

import { MESSAGE_MAX } from './const';
import type { EnquiryForm, EnquiryKind } from './types';

export const emptyEnquiry = (kind: EnquiryKind): EnquiryForm => ({
  kind,
  name: '',
  email: '',
  phone: '',
  message: '',
  eventType: 'Wedding',
  month: '',
  guests: '',
  nights: '',
  budget: '',
});

export function validateEnquiry(f: EnquiryForm): Record<string, string> {
  const errors: Record<string, string> = {};
  if (!f.email.trim() && !f.phone.trim()) errors.email = 'Add an email or a phone number so we can reply.';
  if (f.kind === 'GENERAL' && !f.message.trim()) errors.message = 'Tell us what you’d like to know.';
  if (f.message.length > MESSAGE_MAX) errors.message = `Please keep this under ${MESSAGE_MAX} characters.`;
  if (f.kind === 'EVENT' && f.guests && !(Number(f.guests) > 0)) errors.guests = 'Roughly how many guests?';
  return errors;
}

const toInt = (s: string) => (s.trim() ? Math.max(0, Math.round(Number(s.replace(/[^\d.]/g, '')))) || 0 : 0);

export function toEnquiryRequest(f: EnquiryForm): CreateEnquiryRequest {
  const base = { name: f.name.trim(), email: f.email.trim() || undefined, phone: f.phone.trim() || undefined, message: f.message.trim() || undefined };
  if (f.kind === 'GENERAL') return { ...base, kind: 'GENERAL' };
  return {
    ...base,
    kind: 'EVENT',
    event: {
      type: f.eventType,
      month: f.month || undefined,
      guest_count: toInt(f.guests) || undefined,
      nights: toInt(f.nights) || undefined,
      budget_paise: toInt(f.budget) ? rupeesToPaise(toInt(f.budget)) : undefined,
    },
  };
}
