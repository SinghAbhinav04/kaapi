import Link from 'next/link';

import { BOOKING_STEPS } from '../const';
import type { BookingStep } from '../types';
import styles from './BookingFlow.module.css';

interface StepsProps {
  current: BookingStep;
  /** Where "Room & dates" goes back to, once passed. */
  roomHref?: string;
}

export function Steps({ current, roomHref }: StepsProps) {
  return (
    <ol aria-label="Booking steps" className={styles.steps}>
      {BOOKING_STEPS.map((label, i) => {
        const step = (i + 1) as BookingStep;
        const done = step < current;
        return (
          <li key={label} aria-current={step === current ? 'step' : undefined} className={styles.step} data-state={done ? 'done' : step === current ? 'current' : 'next'}>
            <span className="mono">{done ? '✓' : step}</span>
            {done && step === 1 && roomHref ? <Link href={roomHref}>{label}</Link> : label}
          </li>
        );
      })}
    </ol>
  );
}
