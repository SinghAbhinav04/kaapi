import { cx } from '@/shared/lib/cx';

import styles from './Pill.module.css';
import type { PillProps } from './types';

export function Pill({ tone = 'neutral', mono, className, ...rest }: PillProps) {
  return <span className={cx(styles.pill, styles[tone], mono && styles.mono, className)} {...rest} />;
}
