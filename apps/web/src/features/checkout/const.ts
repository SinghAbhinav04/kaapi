/** ISO 3166 alpha-2, as the API expects. India first; the rest are the resort's usual markets. */
export const COUNTRIES = [
  { value: 'IN', label: 'India' },
  { value: 'GB', label: 'United Kingdom' },
  { value: 'US', label: 'United States' },
  { value: 'DE', label: 'Germany' },
  { value: 'AE', label: 'United Arab Emirates' },
  { value: 'SG', label: 'Singapore' },
  { value: 'AU', label: 'Australia' },
  { value: 'FR', label: 'France' },
  { value: 'NL', label: 'Netherlands' },
  { value: 'CA', label: 'Canada' },
] as const;

export const ARRIVAL_WINDOWS = [
  { value: '2 pm – 4 pm', label: '2 pm – 4 pm' },
  { value: '4 pm – 7 pm', label: '4 pm – 7 pm' },
  { value: 'After 7 pm', label: 'After 7 pm' },
  { value: 'Not sure yet', label: 'Not sure yet' },
] as const;

/** Limits the API enforces too (apps/api/internal/booking). */
export const LIMITS = {
  nameMin: 2,
  nameMax: 120,
  phoneMinDigits: 8,
  phoneMaxDigits: 15,
  requestsMax: 1000,
} as const;

export const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]{2,}$/;

export const OTP_LENGTH = 6;

/** The email step: a code proves the address is real before anything is paid. */
export const EMAIL_COPY = {
  before: 'We\u2019ll send a 6-digit code to check it: your confirmation and reminders go here.',
  after: 'Check your inbox (and spam). The code works for 10 minutes.',
  demo: 'Demo mode, no email sent. Your code:',
  verified: 'Verified. Your confirmation, reminders and any refund go here.',
  needed: 'Verify your email to continue to payment.',
} as const;
