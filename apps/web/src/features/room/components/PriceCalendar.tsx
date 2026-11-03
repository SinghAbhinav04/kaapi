'use client';

import { useState } from 'react';

import { useStaySearch } from '@/features/stay-search';
import { addMonths, firstOfMonth, formatDay, formatMonth, todayIST } from '@/shared/lib/dates';
import { cx } from '@/shared/lib/cx';
import { ErrorState, Skeleton } from '@/shared/ui';

import { CALENDAR_MAX_MONTHS_AHEAD, SCARCITY_THRESHOLD } from '../const';
import { useCalendarMonth } from '../hooks';
import { buildCalendar, nextSelection } from '../lib';
import styles from './PriceCalendar.module.css';

const WEEKDAYS = ['MON', 'TUE', 'WED', 'THU', 'FRI', 'SAT', 'SUN'] as const;

export function PriceCalendar({ slug }: { slug: string }) {
  const { search, setSearch } = useStaySearch();
  const today = todayIST();
  const { month, setMonth, query } = useCalendarMonth(slug, search.checkIn);
  const [pendingCheckIn, setPendingCheckIn] = useState<string | null>(null);
  const cells = buildCalendar(month, query.data?.days ?? [], today, { ...search, pendingCheckIn });
  const lastMonth = addMonths(firstOfMonth(today), CALENDAR_MAX_MONTHS_AHEAD);

  const pick = (date: string) => {
    const next = nextSelection(date, { ...search, pendingCheckIn });
    setPendingCheckIn(next.pendingCheckIn);
    if (next.commit) setSearch({ ...search, ...next.commit });
  };

  return (
    <section aria-labelledby="cal-title" className={styles.section}>
      <div className={styles.head}>
        <h2 id="cal-title" className={styles.title}>
          Nightly prices, {formatMonth(month)}
        </h2>
        <div className={styles.nav}>
          <button type="button" aria-label="Previous month" disabled={month <= firstOfMonth(today)} onClick={() => setMonth(addMonths(month, -1))}>
            ‹
          </button>
          <button type="button" aria-label="Next month" disabled={month >= lastMonth} onClick={() => setMonth(addMonths(month, 1))}>
            ›
          </button>
        </div>
      </div>
      <p className={styles.caption}>ROOM ONLY · BEFORE GST · ₹ IN THOUSANDS</p>

      {query.isError && !query.data ? (
        <ErrorState compact message={query.error.message} onRetry={() => query.refetch()} retrying={query.isFetching} />
      ) : (
        <div className={cx(styles.grid, query.isPlaceholderData && styles.stale)} aria-label={formatMonth(month)} aria-busy={query.isFetching}>
          {WEEKDAYS.map((d) => (
            <span key={d} className={styles.weekday} aria-hidden="true">
              {d}
            </span>
          ))}
          {query.isPending
            ? Array.from({ length: 35 }, (_, i) => <Skeleton key={i} height={64} radius={10} />)
            : cells.map((cell, i) =>
                cell ? (
                  <button
                    key={cell.date}
                    type="button"
                    className={cx(styles.day, cell.season && styles.season, styles[cell.state])}
                    aria-label={cell.label}
                    aria-pressed={cell.state === 'night' || cell.state === 'pending' || undefined}
                    disabled={cell.state === 'past' || (cell.state === 'closed' && !pendingCheckIn)}
                    onClick={() => pick(cell.date)}
                  >
                    <span className={styles.num}>{cell.day}</span>
                    <span className={styles.price}>
                      {cell.state === 'checkout'
                        ? 'OUT'
                        : cell.price + (cell.left !== null && cell.left > 0 && cell.left <= SCARCITY_THRESHOLD ? ` · ${cell.left} left` : '')}
                    </span>
                  </button>
                ) : (
                  <span key={`pad-${i}`} aria-hidden="true" />
                ),
              )}
        </div>
      )}

      <div className={styles.legend}>
        {pendingCheckIn ? (
          <span className={styles.prompt} role="status">
            Check-in {formatDay(pendingCheckIn)}. Now pick your check-out day.
          </span>
        ) : (
          <>
            <span className={styles.key}>
              <span className={cx(styles.swatch, styles.swatchSeason)} />
              Festival and peak dates, minimum stays apply
            </span>
            <span className={styles.key}>
              <span className={cx(styles.swatch, styles.swatchStay)} />
              Your nights
            </span>
            <span>Fridays and Saturdays are 15% higher.</span>
          </>
        )}
      </div>
    </section>
  );
}
