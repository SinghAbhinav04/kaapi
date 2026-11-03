'use client';

import { ErrorState } from '@/shared/ui';

import styles from './error.module.css';

/** Last-resort boundary: something threw while rendering. Offers a retry instead of a blank page. */
export default function Error({ error, reset }: { error: Error & { digest?: string }; reset: () => void }) {
  return (
    <main className={styles.main}>
      <ErrorState title="Something went wrong on this page" message="Please try again. If it keeps happening, tell us the reference below." requestId={error.digest} onRetry={reset} />
    </main>
  );
}
