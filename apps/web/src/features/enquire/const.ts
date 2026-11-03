export const ENQUIRY_KINDS = [
  { value: 'GENERAL', label: 'A question or a stay' },
  { value: 'EVENT', label: 'A wedding or event' },
] as const;

export const EVENT_TYPES = [
  { value: 'Wedding', label: 'Wedding' },
  { value: 'Engagement or reception', label: 'Engagement or reception' },
  { value: 'Company offsite', label: 'Company offsite' },
  { value: 'Reunion or celebration', label: 'Reunion or celebration' },
  { value: 'Something else', label: 'Something else' },
] as const;

export const MESSAGE_MAX = 5000;
