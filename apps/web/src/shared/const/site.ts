/** "Kaapi Estate" is a working name: the brief only says "Company A" (labelled assumption). */
export const SITE = {
  name: 'Kaapi Estate',
  tagline: 'A working coffee estate in Chikkamagaluru',
  location: 'Baba Budan Hills · Chikkamagaluru, Karnataka',
  checkIn: '2 pm',
  checkOut: '11 am',
  holdMinutes: 10,
} as const;

/** Party limits the API also enforces (/v1/availability). */
export const PARTY_LIMITS = { rooms: { min: 1, max: 4 }, adults: { min: 1, max: 16 }, children: { min: 0, max: 8 } } as const;

export const DEFAULT_PARTY = { rooms: 1, adults: 2, children: 0 } as const;
