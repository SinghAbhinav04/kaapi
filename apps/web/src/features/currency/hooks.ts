'use client';

import { useSyncExternalStore } from 'react';

import { useFxRates, type Currency } from '@/apis/catalog';

import { CHARGED_IN, currencyForTimeZone } from './geo';
import { approximate, currencyCookie, readCurrencyCookie } from './lib';

const listeners = new Set<() => void>();

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

// The cookie (set by the guest, or by src/proxy.ts from the visitor's country) wins; with
// neither, a guess from the browser's time zone.
function snapshot(): Currency {
  return readCurrencyCookie(document.cookie) ?? currencyForTimeZone(Intl.DateTimeFormat().resolvedOptions().timeZone);
}

/**
 * The currency the guest reads prices in. Null on the server and during hydration, so the
 * first render matches the server's INR-only page; the estimate appears right after.
 */
export function useDisplayCurrency(): Currency | null {
  return useSyncExternalStore(subscribe, snapshot, () => null);
}

export function setDisplayCurrency(currency: Currency) {
  document.cookie = currencyCookie(currency);
  for (const listener of listeners) listener();
}

/** "$102" for an INR amount, or null when the guest reads INR or rates aren't available. */
export function useApprox(paise: number): string | null {
  const currency = useDisplayCurrency();
  const wanted = currency !== null && currency !== CHARGED_IN;
  const { data } = useFxRates(wanted);
  return approximate(paise, currency, data?.rates);
}
