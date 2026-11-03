import Link from 'next/link';

import { cx } from '@/shared/lib/cx';

import styles from './Button.module.css';
import type { ButtonLinkProps, ButtonProps } from './types';

function classes(p: Pick<ButtonProps, 'variant' | 'size' | 'pill' | 'block' | 'className'>) {
  return cx(styles.button, styles[p.variant ?? 'primary'], styles[p.size ?? 'md'], p.pill && styles.pill, p.block && styles.block, p.className);
}

export function Button({ variant, size, pill, block, loading, icon, className, children, disabled, type = 'button', ...rest }: ButtonProps) {
  return (
    <button
      type={type}
      className={classes({ variant, size, pill, block, className })}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      {...rest}
    >
      {loading ? <span className={styles.spinner} aria-hidden="true" /> : icon}
      {children}
    </button>
  );
}

export function ButtonLink({ variant, size, pill, block, icon, className, children, href, prefetch, ...rest }: ButtonLinkProps) {
  return (
    <Link href={href} prefetch={prefetch} className={classes({ variant, size, pill, block, className })} {...rest}>
      {icon}
      {children}
    </Link>
  );
}
