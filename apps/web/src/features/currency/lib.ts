import type { Currency, FxRate } from '@/apis/catalog';

import { CHARGED_IN, CURRENCY_COOKIE, CURRENCY_COOKIE_MAX_AGE_S, isCurrency } from './geo';

/** paise ÷ 100 ÷ (micros ÷ 1,000,000), kept in integers until the one division. */
const PAISE_MICROS_FACTOR = 10_000;

/** An INR amount in another currency's major units (dollars, euros…), unrounded. */
export function convertPaise(paise: number, inrPerUnitMicros: number): number {
  return (paise * PAISE_MICROS_FACTOR) / inrPerUnitMicros;
}

const formatters = new Map<Currency, Intl.NumberFormat>();

/** "$102", "€88", "¥15,640": whole units, since it's an estimate. */
export function formatForeign(amount: number, currency: Currency): string {
  let f = formatters.get(currency);
  if (!f) {
    f = new Intl.NumberFormat('en', { style: 'currency', currency, maximumFractionDigits: 0 });
    formatters.set(currency, f);
  }
  return f.format(amount);
}

/** The estimate for paise in currency, or null when there's nothing to add (INR, or no rate). */
export function approximate(paise: number, currency: Currency | null, rates: readonly FxRate[] | undefined): string | null {
  if (!currency || currency === CHARGED_IN) return null;
  const rate = rates?.find((r) => r.currency === currency);
  return rate ? formatForeign(convertPaise(paise, rate.inr_per_unit_micros), currency) : null;
}

export function readCurrencyCookie(cookie: string): Currency | null {
  for (const part of cookie.split(';')) {
    const [name, value] = part.trim().split('=');
    if (name === CURRENCY_COOKIE) return isCurrency(value) ? value : null;
  }
  return null;
}

export function currencyCookie(currency: Currency): string {
  return `${CURRENCY_COOKIE}=${currency}; Path=/; Max-Age=${CURRENCY_COOKIE_MAX_AGE_S}; SameSite=Lax`;
}
