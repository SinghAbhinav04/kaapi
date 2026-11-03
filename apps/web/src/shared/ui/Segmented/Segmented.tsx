import { cx } from '@/shared/lib/cx';

import styles from './Segmented.module.css';
import type { SegmentedProps } from './types';

/** A small set of mutually exclusive choices, as toggle buttons. */
export function Segmented<V extends string>({ label, options, value, onChange, tone = 'light' }: SegmentedProps<V>) {
  return (
    <div role="group" aria-label={label} className={cx(styles.group, tone === 'dark' && styles.dark)}>
      {options.map((o) => (
        <button key={o.value} type="button" className={styles.option} aria-pressed={o.value === value} onClick={() => onChange(o.value)}>
          {o.label}
        </button>
      ))}
    </div>
  );
}
