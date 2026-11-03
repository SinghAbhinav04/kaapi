import type { HTMLAttributes } from 'react';

import { cx } from '@/shared/lib/cx';

import styles from './Card.module.css';

/** The console's raised white surface. */
export function Card({ className, dark, ...rest }: HTMLAttributes<HTMLElement> & { dark?: boolean }) {
  return <section className={cx(styles.card, dark && styles.dark, className)} {...rest} />;
}
