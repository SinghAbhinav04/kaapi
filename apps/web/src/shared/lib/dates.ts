/**
 * Stay dates are calendar dates at the resort (IST), passed around as "YYYY-MM-DD"
 * strings. They are parsed as UTC midnight so a guest's own time zone can never shift a
 * night by one. Timestamps (received_at, expires_at) are shown in IST.
 */
export type IsoDate = string;

const RESORT_TZ = 'Asia/Kolkata';
const DAY_MS = 86_400_000;

const toDate = (iso: IsoDate) => new Date(`${iso}T00:00:00Z`);
const fmt = (options: Intl.DateTimeFormatOptions) => new Intl.DateTimeFormat('en-GB', { timeZone: 'UTC', ...options });

const weekdayDayMonth = fmt({ weekday: 'short', day: 'numeric', month: 'short' });
const weekdayDay = fmt({ weekday: 'short', day: 'numeric' });
const dayMonth = fmt({ day: 'numeric', month: 'short' });
const monthYear = fmt({ month: 'long', year: 'numeric' });
const istDateTime = new Intl.DateTimeFormat('en-GB', { timeZone: RESORT_TZ, day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit', hour12: false });
const istTime = new Intl.DateTimeFormat('en-GB', { timeZone: RESORT_TZ, hour: '2-digit', minute: '2-digit', hour12: false });
const istIso = new Intl.DateTimeFormat('en-CA', { timeZone: RESORT_TZ, year: 'numeric', month: '2-digit', day: '2-digit' });

/** Today's date at the resort. */
export function todayIST(now: Date = new Date()): IsoDate {
  return istIso.format(now);
}

export function addDays(iso: IsoDate, days: number): IsoDate {
  return new Date(toDate(iso).getTime() + days * DAY_MS).toISOString().slice(0, 10);
}

export function nightsBetween(checkIn: IsoDate, checkOut: IsoDate): number {
  return Math.round((toDate(checkOut).getTime() - toDate(checkIn).getTime()) / DAY_MS);
}

export function isValidIsoDate(value: string | null | undefined): value is IsoDate {
  return !!value && /^\d{4}-\d{2}-\d{2}$/.test(value) && !Number.isNaN(toDate(value).getTime());
}

/** "Sat 14 Nov" */
export const formatDay = (iso: IsoDate) => weekdayDayMonth.format(toDate(iso)).replace(',', '');
/** "Sat 14" */
export const formatWeekdayDay = (iso: IsoDate) => weekdayDay.format(toDate(iso)).replace(',', '');
/** "14 Nov" */
export const formatDayMonth = (iso: IsoDate) => dayMonth.format(toDate(iso));
/** "November 2026" */
export const formatMonth = (iso: IsoDate) => monthYear.format(toDate(iso));

/** "Sat 14 – Mon 16 Nov", collapsing the month when both dates share it. */
export function formatStay(checkIn: IsoDate, checkOut: IsoDate): string {
  const sameMonth = checkIn.slice(0, 7) === checkOut.slice(0, 7);
  return `${sameMonth ? formatWeekdayDay(checkIn) : formatDay(checkIn)} – ${formatDay(checkOut)}`;
}

/** "14–16 Nov" */
export function formatStayShort(checkIn: IsoDate, checkOut: IsoDate): string {
  const sameMonth = checkIn.slice(0, 7) === checkOut.slice(0, 7);
  return sameMonth ? `${toDate(checkIn).getUTCDate()}–${formatDayMonth(checkOut)}` : `${formatDayMonth(checkIn)} – ${formatDayMonth(checkOut)}`;
}

/** "23 Oct, 12:30" in IST. */
export const formatTimestamp = (ts: string) => istDateTime.format(new Date(ts));
/** "14:18" in IST. */
export const formatClock = (ts: string) => istTime.format(new Date(ts));

/** "40 min", "2 h 10 min", or the date once it's more than a day old. */
export function formatAge(ts: string, now: Date = new Date()): string {
  const minutes = Math.max(0, Math.round((now.getTime() - new Date(ts).getTime()) / 60_000));
  if (minutes < 60) return `${minutes} min`;
  if (minutes < 24 * 60) {
    const h = Math.floor(minutes / 60);
    const m = minutes % 60;
    return m ? `${h} h ${m} min` : `${h} h`;
  }
  return istDateTime.format(new Date(ts)).split(',')[0] ?? '';
}

/** Monday-first weeks for a month: null pads the days before the 1st. */
export function monthGrid(firstOfMonth: IsoDate): (IsoDate | null)[] {
  const first = toDate(firstOfMonth);
  const lead = (first.getUTCDay() + 6) % 7; // Monday = 0
  const days = new Date(Date.UTC(first.getUTCFullYear(), first.getUTCMonth() + 1, 0)).getUTCDate();
  const cells: (IsoDate | null)[] = Array.from({ length: lead }, () => null);
  for (let d = 0; d < days; d++) cells.push(addDays(firstOfMonth, d));
  return cells;
}

export function firstOfMonth(iso: IsoDate): IsoDate {
  return `${iso.slice(0, 7)}-01`;
}

export function addMonths(firstOfMonthIso: IsoDate, months: number): IsoDate {
  const d = toDate(firstOfMonthIso);
  return new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth() + months, 1)).toISOString().slice(0, 10);
}

/** 0 = Monday … 6 = Sunday */
export function weekdayIndex(iso: IsoDate): number {
  return (toDate(iso).getUTCDay() + 6) % 7;
}

export function dayOfMonth(iso: IsoDate): number {
  return toDate(iso).getUTCDate();
}
