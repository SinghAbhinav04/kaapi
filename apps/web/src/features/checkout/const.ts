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
