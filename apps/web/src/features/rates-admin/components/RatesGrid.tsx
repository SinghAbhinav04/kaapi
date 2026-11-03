'use client';

import Image from 'next/image';

import { useRooms } from '@/apis/catalog';
import type { RatesGridResponse } from '@/apis/rates';
import { dayOfMonth, formatDay, weekdayIndex } from '@/shared/lib/dates';
import { cx } from '@/shared/lib/cx';
import { plural } from '@/shared/lib/text';

import { cellTone, priceK } from '../lib';
import type { GridSelection } from '../types';
import styles from './Rates.module.css';

const WEEKDAY = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'] as const;

interface RatesGridProps {
  grid: RatesGridResponse;
  selection: GridSelection | null;
  onPick: (roomTypeId: string, date: string) => void;
  stale: boolean;
}

export function RatesGrid({ grid, selection, onPick, stale }: RatesGridProps) {
  const rooms = useRooms();
  const photoOf = new Map(rooms.data?.rooms.map((r) => [r.slug, r.photos[0]]));
  const dates = grid.rows[0]?.nights.map((n) => n.date) ?? [];

  return (
    <div className={styles.scroller}>
      <div className={cx(styles.grid, stale && styles.stale)} style={{ gridTemplateColumns: `200px repeat(${dates.length}, minmax(56px, 1fr))` }} role="grid" aria-label="Nightly price and rooms left">
        <span role="columnheader" />
        {dates.map((d) => (
          <span key={d} role="columnheader" className={cx(styles.dayHead, weekdayIndex(d) >= 4 && weekdayIndex(d) <= 5 && styles.weekend)} aria-label={formatDay(d)}>
            {WEEKDAY[weekdayIndex(d)]}
            <b>{dayOfMonth(d)}</b>
          </span>
        ))}
        {grid.rows.map((row) => {
          const photo = photoOf.get(row.slug);
          const online = row.nights[0]?.allotment ?? 0;
          return (
            <div key={row.room_type_id} role="row" className={styles.rowContents}>
              <div role="rowheader" className={styles.rowHead}>
                {photo && <Image src={photo.url} alt="" width={44} height={44} className={styles.rowPhoto} />}
                <div>
                  <strong>{row.name}</strong>
                  <span>
                    {online} of {row.total_keys} keys online
                  </span>
                </div>
              </div>
              {row.nights.map((n) => {
                const selected = selection?.roomTypeId === row.room_type_id && n.date >= selection.first && n.date <= selection.last;
                const tone = cellTone(n);
                return (
                  <button
                    key={n.date}
                    type="button"
                    role="gridcell"
                    className={cx(styles.cell, styles[tone], selected && styles.selected)}
                    aria-selected={selected}
                    aria-label={`${row.name}, ${formatDay(n.date)}: ₹${(n.price_paise / 100).toLocaleString('en-IN')}, ${n.stop_sell ? 'stopped' : `${plural(n.left, 'room')} left`}${n.held ? `, ${n.held} on hold` : ''}`}
                    onClick={() => onPick(row.room_type_id, n.date)}
                  >
                    <b>{priceK(n.price_paise)}</b>
                    <span>{n.stop_sell ? 'Stopped' : n.left <= 0 ? 'Full' : `${n.left} left`}</span>
                    {n.min_stay > 1 && <i className={styles.minStay}>{n.min_stay}+</i>}
                  </button>
                );
              })}
            </div>
          );
        })}
      </div>
    </div>
  );
}
