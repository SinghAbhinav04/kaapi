import type { PillTone } from '@/shared/ui';

export interface QuoteRow {
  room: string;
  roomOnly: string;
  breakfast: string | null;
  left: number | null;
}

/** A draft split into prose and the server-written quote, for display. */
export type DraftBlock = { kind: 'text'; text: string } | { kind: 'quote'; head: string | null; rows: QuoteRow[] };

export interface ItemBadge {
  label: string;
  tone: PillTone;
}
