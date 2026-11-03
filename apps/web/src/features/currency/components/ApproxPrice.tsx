'use client';

import { cx } from '@/shared/lib/cx';

import { COPY } from '../const';
import { useApprox } from '../hooks';
import styles from './Currency.module.css';

interface ApproxPriceProps {
  paise: number;
  /** Adds "at today's rate · you're charged in INR", for the amount actually paid. */
  withNote?: boolean;
  className?: string;
}

/** "≈ $102" beside an INR price, in the guest's currency. Renders nothing for INR. */
export function ApproxPrice({ paise, withNote = false, className }: ApproxPriceProps) {
  const text = useApprox(paise);
  if (!text) return null;
  return (
    <span className={cx(styles.approx, className)} title={COPY.approxTitle}>
      <span aria-hidden="true">≈ </span>
      <span className="sr-only">{COPY.approxPrefix} </span>
      {text}
      {withNote && <span className={styles.note}> {COPY.chargedNote}</span>}
    </span>
  );
}
