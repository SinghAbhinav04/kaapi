import { cx } from '@/shared/lib/cx';

import { Button } from '../Button';
import styles from './State.module.css';
import type { EmptyStateProps, ErrorStateProps } from './types';

/** Nothing to show yet, said plainly, with the next step when there is one. */
export function EmptyState({ title, children, action, compact }: EmptyStateProps) {
  return (
    <div className={cx(styles.state, compact && styles.compact)}>
      <p className={styles.title}>{title}</p>
      {children && <div className={styles.body}>{children}</div>}
      {action}
    </div>
  );
}

/** Something failed: what happened, a way to retry, and a reference for support. */
export function ErrorState({ title = 'That didn’t load', message, requestId, onRetry, retrying, compact }: ErrorStateProps) {
  return (
    <div role="alert" className={cx(styles.state, styles.error, compact && styles.compact)}>
      <p className={styles.title}>{title}</p>
      <p className={styles.body}>{message}</p>
      {onRetry && (
        <Button variant="outline" size="sm" pill onClick={onRetry} loading={retrying}>
          Try again
        </Button>
      )}
      {requestId && <span className={styles.ref}>REF {requestId}</span>}
    </div>
  );
}
