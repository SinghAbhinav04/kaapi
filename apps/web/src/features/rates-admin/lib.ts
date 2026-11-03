import type { RateNight, RateRow, RatesGridResponse, UpdateRatesRequest } from '@/apis/rates';
import { addDays, weekdayIndex, type IsoDate } from '@/shared/lib/dates';
import { paiseToRupees, rupeesToPaise } from '@/shared/lib/money';

import type { CellTone, GridSelection, RateEdit } from './types';

/** Monday of the week containing `date`. */
export const weekStart = (date: IsoDate) => addDays(date, -weekdayIndex(date));

/** Heat-map colour: how much of the website's allotment that night has gone. */
export function cellTone(n: RateNight): CellTone {
  if (n.stop_sell) return 'closed';
  if (n.left <= 0) return 'full';
  const taken = n.sold + n.held;
  if (taken === 0) return 'free';
  return taken * 2 >= n.allotment ? 'half' : 'some';
}

/** 8.2k: rupees in thousands, one decimal. */
export const priceK = (paise: number) => `${(paise / 100_000).toFixed(1)}k`;

export function selectedNights(row: RateRow | undefined, sel: GridSelection | null): RateNight[] {
  if (!row || !sel || row.room_type_id !== sel.roomTypeId) return [];
  return row.nights.filter((n) => n.date >= sel.first && n.date <= sel.last);
}

/** Clicking a night: same row extends the range, otherwise starts a new one. */
export function nextSelection(sel: GridSelection | null, roomTypeId: string, date: IsoDate): GridSelection | null {
  if (sel && sel.roomTypeId === roomTypeId) {
    if (sel.first === date && sel.last === date) return null; // click again to clear
    if (sel.first === sel.last) return date < sel.first ? { roomTypeId, first: date, last: sel.first } : { roomTypeId, first: sel.first, last: date };
  }
  return { roomTypeId, first: date, last: date };
}

/** What the editor starts with: the shared value, or blank when nights differ. */
export function initialEdit(nights: readonly RateNight[]): RateEdit {
  const same = <T,>(pick: (n: RateNight) => T): T | null => {
    const first = nights[0];
    if (!first) return null;
    const v = pick(first);
    return nights.every((n) => pick(n) === v) ? v : null;
  };
  const price = same((n) => n.price_paise);
  return {
    priceRupees: price === null ? '' : String(paiseToRupees(price)),
    allotment: same((n) => n.allotment),
    minStay: same((n) => n.min_stay),
    stopSell: same((n) => n.stop_sell),
  };
}

/** The lowest allotment the API will accept: rooms already sold or on hold. */
export const minimumAllotment = (nights: readonly RateNight[]) => nights.reduce((m, n) => Math.max(m, n.sold + n.held), 0);

/** Sends only what changed, so a stop-sell doesn't also rewrite prices. */
export function toUpdate(sel: GridSelection, start: RateEdit, edit: RateEdit): UpdateRatesRequest | null {
  const body: UpdateRatesRequest = { room_type_id: sel.roomTypeId, from: sel.first, to: addDays(sel.last, 1) };
  let changed = false;
  const price = Number(edit.priceRupees.replace(/[,\s]/g, ''));
  if (edit.priceRupees.trim() && edit.priceRupees !== start.priceRupees && price > 0) {
    body.room_only_price_paise = rupeesToPaise(price);
    changed = true;
  }
  if (edit.allotment !== null && edit.allotment !== start.allotment) {
    body.allotment = edit.allotment;
    changed = true;
  }
  if (edit.minStay !== null && edit.minStay !== start.minStay) {
    body.min_stay = edit.minStay;
    changed = true;
  }
  if (edit.stopSell !== null && edit.stopSell !== start.stopSell) {
    body.stop_sell = edit.stopSell;
    changed = true;
  }
  return changed ? body : null;
}

/** The four numbers above the grid. */
export function gridKpis(grid: RatesGridResponse | undefined) {
  let sold = 0;
  let allotment = 0;
  let held = 0;
  const soldOut: { room: string; date: IsoDate }[] = [];
  for (const row of grid?.rows ?? []) {
    for (const n of row.nights) {
      sold += n.sold;
      allotment += n.allotment;
      held += n.held;
      if (n.left <= 0 && !n.stop_sell) soldOut.push({ room: row.name, date: n.date });
    }
  }
  return { sold, allotment, held, soldOut };
}
