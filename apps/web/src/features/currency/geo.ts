import type { Currency } from '@/apis/catalog';

/**
 * Pure and dependency-free, so src/proxy.ts can use it on every request without loading
 * any React code.
 */

/** Remembers the guest's choice (readable by the page, so not httpOnly). */
export const CURRENCY_COOKIE = 'kaapi_ccy';
export const CURRENCY_COOKIE_MAX_AGE_S = 60 * 60 * 24 * 365;

/** Every booking is charged in this, whatever the guest reads prices in. */
export const CHARGED_IN: Currency = 'INR';

export const CURRENCIES: readonly Currency[] = ['INR', 'USD', 'EUR', 'GBP', 'AUD', 'CAD', 'SGD', 'CHF', 'JPY'];

export function isCurrency(value: unknown): value is Currency {
  return typeof value === 'string' && (CURRENCIES as readonly string[]).includes(value);
}

const EUROZONE = ['AT', 'BE', 'CY', 'DE', 'EE', 'ES', 'FI', 'FR', 'GR', 'HR', 'IE', 'IT', 'LT', 'LU', 'LV', 'MT', 'NL', 'PT', 'SI', 'SK'];

/** Country (ISO 3166 alpha-2, as hosting edges report it) → the currency shown on a first visit. */
const COUNTRY_CURRENCY: Readonly<Record<string, Currency>> = {
  ...Object.fromEntries(EUROZONE.map((c) => [c, 'EUR' as const])),
  US: 'USD',
  GB: 'GBP',
  AU: 'AUD',
  CA: 'CAD',
  SG: 'SGD',
  CH: 'CHF',
  LI: 'CHF',
  JP: 'JPY',
};

/** Anywhere else, India included, starts in INR; the guest can switch. */
export function currencyForCountry(country: string | null | undefined): Currency {
  return (country && COUNTRY_CURRENCY[country.toUpperCase()]) || CHARGED_IN;
}

const EUROZONE_CITIES = ['Berlin', 'Paris', 'Madrid', 'Rome', 'Amsterdam', 'Brussels', 'Vienna', 'Dublin', 'Lisbon', 'Helsinki', 'Athens', 'Luxembourg'];

/**
 * The fallback when no country was reported (local development, another host): a guess from
 * the browser's time zone. Rough on purpose; a time zone isn't a currency, so the switcher
 * always wins and specific zones are listed before broad regions.
 */
const TIME_ZONE_CURRENCY: readonly (readonly [prefix: string, currency: Currency])[] = [
  ['Asia/Kolkata', 'INR'],
  ['Asia/Calcutta', 'INR'],
  ['Europe/London', 'GBP'],
  ['Europe/Zurich', 'CHF'],
  ['Asia/Singapore', 'SGD'],
  ['Asia/Tokyo', 'JPY'],
  ['Australia/', 'AUD'],
  ['America/Toronto', 'CAD'],
  ['America/Vancouver', 'CAD'],
  ['America/Edmonton', 'CAD'],
  ['America/Winnipeg', 'CAD'],
  ['America/Halifax', 'CAD'],
  ['America/New_York', 'USD'],
  ['America/Chicago', 'USD'],
  ['America/Denver', 'USD'],
  ['America/Phoenix', 'USD'],
  ['America/Los_Angeles', 'USD'],
  ['America/Anchorage', 'USD'],
  ['Pacific/Honolulu', 'USD'],
  ...EUROZONE_CITIES.map((city) => [`Europe/${city}`, 'EUR'] as const),
];

export function currencyForTimeZone(timeZone: string | undefined): Currency {
  if (!timeZone) return CHARGED_IN;
  return TIME_ZONE_CURRENCY.find(([prefix]) => timeZone.startsWith(prefix))?.[1] ?? CHARGED_IN;
}
