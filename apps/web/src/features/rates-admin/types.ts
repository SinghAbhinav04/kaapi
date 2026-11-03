import type { IsoDate } from '@/shared/lib/dates';

/** Nights picked in one room type's row, inclusive. */
export interface GridSelection {
  roomTypeId: string;
  first: IsoDate;
  last: IsoDate;
}

export type CellTone = 'free' | 'some' | 'half' | 'full' | 'closed';

/** The editor's fields; null means "leave as it is". */
export interface RateEdit {
  priceRupees: string;
  allotment: number | null;
  minStay: number | null;
  stopSell: boolean | null;
}
