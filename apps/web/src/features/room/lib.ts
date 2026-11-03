import type { CalendarDay, QuoteLine } from '@/apis/catalog';
import { dayOfMonth, formatDay, monthGrid, type IsoDate } from '@/shared/lib/dates';
import { formatThousands } from '@/shared/lib/money';
import { plural } from '@/shared/lib/text';

import type { CalendarCell, DayState } from './types';

interface Selection {
  checkIn: IsoDate;
  checkOut: IsoDate;
  /** Set between the first and second click. */
  pendingCheckIn: IsoDate | null;
}

function stateOf(date: IsoDate, info: CalendarDay | undefined, today: IsoDate, sel: Selection): DayState {
  if (date < today) return 'past';
  if (sel.pendingCheckIn) {
    if (date === sel.pendingCheckIn) return 'pending';
  } else {
    if (date >= sel.checkIn && date < sel.checkOut) return 'night';
    if (date === sel.checkOut) return 'checkout';
  }
  if (!info || info.stop_sell || info.left === 0) return 'closed';
  return 'open';
}

/** Month cells (Monday-first, null pads) with what each day shows. Pure. */
export function buildCalendar(firstOfMonth: IsoDate, days: readonly CalendarDay[], today: IsoDate, sel: Selection): (CalendarCell | null)[] {
  const byDate = new Map(days.map((d) => [d.date, d]));
  return monthGrid(firstOfMonth).map((date) => {
    if (!date) return null;
    const info = byDate.get(date);
    const state = stateOf(date, info, today, sel);
    const left = info && !info.stop_sell ? info.left : null;
    const parts = [formatDay(date)];
    if (info && state !== 'past') {
      parts.push(info.stop_sell || info.left === 0 ? 'sold out online' : `₹${formatThousands(info.price_paise)} thousand, ${plural(info.left, 'room')} left`);
      if (info.min_stay > 1) parts.push(`${info.min_stay}-night minimum`);
      if (info.no_arrival) parts.push('no arrivals');
    }
    if (state === 'night') parts.push('in your stay');
    if (state === 'checkout') parts.push('check-out');
    return {
      date,
      day: dayOfMonth(date),
      state,
      price: info && !info.stop_sell && info.left > 0 ? formatThousands(info.price_paise) : '—',
      left,
      season: info?.season ?? false,
      minStay: info?.min_stay ?? 1,
      noArrival: info?.no_arrival ?? false,
      label: parts.join(', '),
    };
  });
}

/** The next selection after clicking a day: first click picks check-in, second check-out. */
export function nextSelection(clicked: IsoDate, sel: Selection): { pendingCheckIn: IsoDate | null; commit: { checkIn: IsoDate; checkOut: IsoDate } | null } {
  if (sel.pendingCheckIn && clicked > sel.pendingCheckIn) {
    return { pendingCheckIn: null, commit: { checkIn: sel.pendingCheckIn, checkOut: clicked } };
  }
  return { pendingCheckIn: clicked, commit: null };
}

/** GST is charged per night at that night's slab; summarise it for the total line. */
export function gstSummary(lines: readonly QuoteLine[]): string {
  const rates = [...new Set(lines.map((l) => l.gst_rate_bps))];
  if (rates.length === 1) {
    const rate = (rates[0] ?? 0) / 100;
    return lines.length > 1 ? `GST at ${rate}%, every night` : `GST at ${rate}%`;
  }
  return 'GST at 5% and 18%, night by night';
}
