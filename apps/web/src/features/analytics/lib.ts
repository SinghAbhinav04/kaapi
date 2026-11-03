import { addDays, todayIST, type IsoDate } from '@/shared/lib/dates';

const PAISE_PER_RUPEE = 100;
const LAKH = 100_000;
const CRORE = 10_000_000;
const THOUSAND = 1_000;

/** The booked-date window for a preset: `days` days ending today (IST). */
export function rangeParams(days: number, today: IsoDate = todayIST()): { from: IsoDate; to: IsoDate } {
  return { from: addDays(today, -(days - 1)), to: today };
}

/** Axis-friendly rupees in Indian units: ₹0, ₹50K, ₹1.5L, ₹2Cr. */
export function compactINR(paise: number): string {
  const r = paise / PAISE_PER_RUPEE;
  const fmt = (v: number, unit: string) => `₹${Number.isInteger(v) ? v : v.toFixed(1)}${unit}`;
  if (r >= CRORE) return fmt(r / CRORE, 'Cr');
  if (r >= LAKH) return fmt(r / LAKH, 'L');
  if (r >= THOUSAND) return fmt(r / THOUSAND, 'K');
  return `₹${Math.round(r)}`;
}

/** Clean ticks from 0 to at least `max`: steps of 1, 2 or 5 × 10ⁿ, about `target` of them. */
export function niceTicks(max: number, target: number): number[] {
  if (max <= 0) return [0];
  const raw = max / target;
  const pow = 10 ** Math.floor(Math.log10(raw));
  const step = [1, 2, 5, 10].map((m) => m * pow).find((s) => s >= raw) ?? 10 * pow;
  const ticks: number[] = [];
  for (let v = 0; v < max + step; v += step) ticks.push(v);
  return ticks;
}

export const percent = (share: number | null | undefined, digits = 0) => (share === null || share === undefined ? '—' : `${(share * 100).toFixed(digits)}%`);

/** Change against the previous window, or null when there's nothing to compare with. */
export function change(current: number, previous: number): number | null {
  return previous > 0 ? (current - previous) / previous : null;
}

/** Whole rupees for headline tiles: ₹19,70,000 rather than ₹19,69,999.20. */
export const roundToRupee = (paise: number) => Math.round(paise / PAISE_PER_RUPEE) * PAISE_PER_RUPEE;
