import Link from 'next/link';

import { cx } from '@/shared/lib/cx';

import styles from './Logo.module.css';
import type { LogoProps } from './types';

export function Logo({ href = '/', size = 'md', className }: LogoProps) {
  return (
    <Link href={href} className={cx(styles.logo, styles[size], className)}>
      KAAPI ESTATE
    </Link>
  );
}
