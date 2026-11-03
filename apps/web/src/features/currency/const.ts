import type { Currency } from '@/apis/catalog';

export const CURRENCY_LABELS: Readonly<Record<Currency, string>> = {
  INR: 'INR ₹',
  USD: 'USD $',
  EUR: 'EUR €',
  GBP: 'GBP £',
  AUD: 'AUD A$',
  CAD: 'CAD C$',
  SGD: 'SGD S$',
  CHF: 'CHF',
  JPY: 'JPY ¥',
};

export const COPY = {
  switcherLabel: 'Show prices in',
  approxPrefix: 'about',
  approxTitle: "An estimate at today's ECB rate. You're charged in INR.",
  chargedNote: "at today's rate · you're charged in INR",
} as const;
