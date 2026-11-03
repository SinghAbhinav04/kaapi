'use client';

import { useState } from 'react';

import { useCountdown } from '@/shared/hooks';
import { cx } from '@/shared/lib/cx';

import styles from './BookingFlow.module.css';

interface HoldTimerProps {
  expiresAt: string;
  onExpire?: () => void;
}

/** "Held for you 09:42". Ticks in the DOM; React re-renders only once, at expiry. */
export function HoldTimer({ expiresAt, onExpire }: HoldTimerProps) {
  const [expired, setExpired] = useState(() => new Date(expiresAt).getTime() <= Date.now());
  const clock = useCountdown<HTMLSpanElement>(expiresAt, () => {
    setExpired(true);
    onExpire?.();
  });
  return (
    <p role="timer" aria-live="off" className={cx(styles.timer, expired && styles.timerExpired)}>
      {expired ? 'Hold ended' : 'Held for you'} <span ref={clock} className={styles.clock} />
    </p>
  );
}
