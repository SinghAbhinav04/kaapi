'use client';

import { useId } from 'react';

import { cx } from '@/shared/lib/cx';

import { CURRENCY_LABELS, COPY } from '../const';
import { CHARGED_IN, CURRENCIES, isCurrency } from '../geo';
import { setDisplayCurrency, useDisplayCurrency } from '../hooks';
import styles from './Currency.module.css';

/** Which currency the "≈" estimates use. Prices themselves, and the charge, stay INR. */
export function CurrencySwitcher({ className }: { className?: string }) {
  const id = useId();
  const currency = useDisplayCurrency() ?? CHARGED_IN;
  return (
    <span className={cx(styles.switcher, className)}>
      <label htmlFor={id} className="sr-only">
        {COPY.switcherLabel}
      </label>
      <select
        id={id}
        value={currency}
        title={COPY.approxTitle}
        onChange={(e) => {
          if (isCurrency(e.target.value)) setDisplayCurrency(e.target.value);
        }}
      >
        {CURRENCIES.map((c) => (
          <option key={c} value={c}>
            {CURRENCY_LABELS[c]}
          </option>
        ))}
      </select>
    </span>
  );
}
