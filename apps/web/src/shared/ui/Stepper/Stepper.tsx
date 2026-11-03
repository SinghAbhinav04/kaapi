import { cx } from '@/shared/lib/cx';

import styles from './Stepper.module.css';
import type { StepperProps } from './types';

/** − value + with an accessible name; the value is announced as it changes. */
export function Stepper({ label, value, min = 0, max = Number.MAX_SAFE_INTEGER, onChange, tone = 'light', warn }: StepperProps) {
  return (
    <span role="group" aria-label={label} className={cx(styles.stepper, tone === 'dark' && styles.dark, warn && styles.warn)}>
      <button type="button" className={styles.step} aria-label={`One fewer: ${label}`} disabled={value <= min} onClick={() => onChange(value - 1)}>
        −
      </button>
      <span className={styles.value} aria-live="polite">
        {value}
      </span>
      <button type="button" className={styles.step} aria-label={`One more: ${label}`} disabled={value >= max} onClick={() => onChange(value + 1)}>
        +
      </button>
    </span>
  );
}
