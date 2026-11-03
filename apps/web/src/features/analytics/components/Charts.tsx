import type { ReactNode } from 'react';

import type { AnalyticsResponse } from '@/apis/analytics';
import { formatDayMonth, formatMonth } from '@/shared/lib/dates';
import { formatINR } from '@/shared/lib/money';
import { plural } from '@/shared/lib/text';

import { DAILY_LABEL_TARGET, TICK_TARGET } from '../const';
import { compactINR, niceTicks, percent } from '../lib';
import styles from './Analytics.module.css';

type Daily = AnalyticsResponse['daily'];
type Months = AnalyticsResponse['on_the_books'];

/** Daily revenue as columns. Each column is a focusable target with its own tooltip. */
export function RevenueColumns({ days }: { days: Daily }) {
  const ticks = niceTicks(Math.max(...days.map((d) => d.revenue_paise), 0), TICK_TARGET);
  const top = ticks[ticks.length - 1] || 1;
  const every = Math.max(1, Math.ceil(days.length / DAILY_LABEL_TARGET));

  return (
    <>
      <div className={styles.plot} role="img" aria-label="Revenue by day booked; the table below has every value">
        {ticks.map((t) => (
          <div key={t} className={styles.gridline} style={{ bottom: `${(t / top) * 100}%` }}>
            <span>{compactINR(t)}</span>
          </div>
        ))}
        <div className={styles.columns}>
          {days.map((d) => (
            <button key={d.date} type="button" className={styles.col} aria-label={`${formatDayMonth(d.date)}: ${formatINR(d.revenue_paise)}`}>
              {d.revenue_paise > 0 && <span className={styles.bar} style={{ height: `${(d.revenue_paise / top) * 100}%` }} />}
              <span className={styles.tip} aria-hidden="true">
                {formatDayMonth(d.date)}
                <strong>{formatINR(d.revenue_paise)}</strong>
                {plural(d.bookings, 'booking')}
              </span>
            </button>
          ))}
        </div>
      </div>
      <div className={styles.xAxis} aria-hidden="true">
        {days.map((d, i) =>
          i % every === 0 ? (
            <span key={d.date} style={{ left: `${((i + 0.5) / days.length) * 100}%` }}>
              {formatDayMonth(d.date)}
            </span>
          ) : null,
        )}
      </div>
      <TableView
        head={['Date', 'Bookings', 'Revenue']}
        rows={days.filter((d) => d.bookings > 0).map((d) => [formatDayMonth(d.date), d.bookings, formatINR(d.revenue_paise)])}
      />
    </>
  );
}

/** Horizontal bars with the value at each tip. */
export function BarList({ rows }: { rows: { key: string; label: string; value: number; display: string }[] }) {
  const max = Math.max(...rows.map((r) => r.value), 0) || 1;
  return (
    <ul className={styles.bars}>
      {rows.map((r) => (
        <li key={r.key} className={styles.barRow}>
          <span className={styles.barName} title={r.label}>
            {r.label}
          </span>
          <span className={styles.barTrack} aria-hidden="true">
            {r.value > 0 && <span className={styles.barFill} style={{ width: `${(r.value / max) * 100}%`, display: 'block' }} />}
          </span>
          <span className={styles.barValue}>{r.display}</span>
        </li>
      ))}
    </ul>
  );
}

/** Each month ahead: nights sold as a share of the website's allotment, plus room revenue. */
export function OnTheBooks({ months }: { months: Months }) {
  const iso = (m: string) => `${m}-01`;
  return (
    <>
      <ul className={styles.bars}>
        {months.map((m) => {
          const share = m.allotment > 0 ? m.sold / m.allotment : null;
          return (
            <li key={m.month} className={styles.barRow}>
              <span className={styles.barName}>{formatMonth(iso(m.month))}</span>
              <span
                className={styles.meter}
                role="meter"
                aria-label={`${formatMonth(iso(m.month))} occupancy`}
                aria-valuemin={0}
                aria-valuemax={m.allotment}
                aria-valuenow={m.sold}
              >
                <span className={styles.meterFill} style={{ width: `${(share ?? 0) * 100}%`, display: 'block' }} />
              </span>
              <span className={styles.barValue}>
                {percent(share)} · {compactINR(m.room_revenue_paise)}
              </span>
            </li>
          );
        })}
      </ul>
      <TableView
        head={['Month', 'Nights sold', 'Allotment', 'Occupancy', 'Room revenue']}
        rows={months.map((m) => [
          formatMonth(iso(m.month)),
          m.sold,
          m.allotment,
          percent(m.allotment ? m.sold / m.allotment : null, 1),
          formatINR(m.room_revenue_paise),
        ])}
      />
    </>
  );
}

/** The same numbers as a table, for screen readers and anyone who wants exact values. */
export function TableView({ head, rows }: { head: string[]; rows: ReactNode[][] }) {
  if (rows.length === 0) return null;
  return (
    <details className={styles.tableView}>
      <summary>Show as table</summary>
      <div className={styles.tableScroll}>
        <table className={styles.table}>
          <thead>
            <tr>
              {head.map((h, i) => (
                <th key={h} scope="col" className={i > 0 ? styles.num : undefined}>
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map((r, i) => (
              <tr key={i}>
                {r.map((c, j) => (
                  <td key={j} className={j > 0 ? styles.num : undefined}>
                    {c}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </details>
  );
}
