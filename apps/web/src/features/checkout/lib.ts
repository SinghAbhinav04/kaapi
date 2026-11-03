import type { CreateBookingRequest } from '@/apis/booking';

import { EMAIL_PATTERN, LIMITS } from './const';
import type { FormErrors, GuestForm } from './types';

/** The same checks the API makes, so most mistakes are caught before a round trip. */
export function validateGuest(form: GuestForm): FormErrors {
  const errors: FormErrors = {};
  const name = form.name.trim();
  if (name.length < LIMITS.nameMin || name.length > LIMITS.nameMax) errors['guest.name'] = 'Please enter your full name.';
  if (!EMAIL_PATTERN.test(form.email.trim())) errors['guest.email'] = 'Please enter an email we can send your confirmation to.';
  const digits = form.phone.replace(/[\s()+-]/g, '');
  if (!/^\d+$/.test(digits) || digits.length < LIMITS.phoneMinDigits || digits.length > LIMITS.phoneMaxDigits) {
    errors['guest.phone'] = 'Please enter a mobile number with its country code, e.g. +91 98450 12345.';
  }
  if (form.specialRequests.length > LIMITS.requestsMax) errors.special_requests = `Please keep this under ${LIMITS.requestsMax} characters.`;
  if (!form.consent) errors.consent = 'We need your consent to use these details for the booking.';
  return errors;
}

export function toBookingRequest(holdId: string, form: GuestForm): CreateBookingRequest {
  return {
    hold_id: holdId,
    guest: { name: form.name.trim(), email: form.email.trim(), phone: form.phone.trim(), country: form.country },
    arrival_window: form.arrivalWindow,
    special_requests: form.specialRequests.trim() || undefined,
    consent: form.consent,
    marketing_opt_in: form.marketing,
  };
}

export const EMPTY_FORM: GuestForm = {
  name: '',
  email: '',
  phone: '',
  country: 'IN',
  arrivalWindow: '2 pm – 4 pm',
  specialRequests: '',
  consent: false,
  marketing: false,
};
