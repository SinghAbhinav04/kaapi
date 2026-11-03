'use client';

import { useState } from 'react';

import { hasPermission, useMe } from '@/apis/auth';
import { useExportDay, useStaffBookings } from '@/apis/booking';
import { stayCalendarFile } from '@/features/confirmation';
import { STATUS_LABEL } from '@/features/my-bookings';
import { Card, StaffPageHeader } from '@/features/staff-shell';
import { formatStayShort, formatTimestamp, todayIST } from '@/shared/lib/dates';
import { cx } from '@/shared/lib/cx';
import { formatINR } from '@/shared/lib/money';
import { formatParty } from '@/shared/lib/text';
import { Button, EmptyState, ErrorState, Pill, Segmented, Skeleton } from '@/shared/ui';

import { ARCHIVE_NOTE, BOOKINGS_LIMIT, STATUS_FILTERS, VIEW_FILTERS } from '../const';
import styles from './StaffBookings.module.css';

type StatusFilter = (typeof STATUS_FILTERS)[number]['value'];
type ViewFilter = (typeof VIEW_FILTERS)[number]['value'];

export function StaffBookingsPage() {
  const [view, setView] = useState<ViewFilter>('live');
  const [status, setStatus] = useState<StatusFilter>('');
  const [from, setFrom] = useState('');
  const [exportDate, setExportDate] = useState(todayIST());
  const bookings = useStaffBookings({ status: status || undefined, from: from || undefined, archived: view === 'archived', limit: BOOKINGS_LIMIT });
  const exportDay = useExportDay();
  const { data: me } = useMe();
  const canExport = hasPermission(me ?? null, 'bookings:export');

  const download = () =>
    exportDay.mutate(exportDate, {
      onSuccess: (csv) => {
        const url = URL.createObjectURL(new Blob([csv], { type: 'text/csv' }));
        Object.assign(document.createElement('a'), { href: url, download: `kaapi-bookings-${exportDate}.csv` }).click();
        URL.revokeObjectURL(url);
      },
    });

  return (
    <div className={styles.page}>
      <StaffPageHeader
        title="Bookings"
        description="Direct bookings from the website. Each day's file goes into the reservation system."
        actions={
          canExport && (
            <div className={styles.export}>
              <label>
                <span className="sr-only">Export date</span>
                <input type="date" value={exportDate} onChange={(e) => setExportDate(e.target.value)} />
              </label>
              <Button variant="ink" onClick={download} loading={exportDay.isPending}>
                Download CSV
              </Button>
            </div>
          )
        }
      />
      {exportDay.isError && (
        <p className={styles.error} role="alert">
          {exportDay.error.message}
        </p>
      )}

      <Card className={styles.card}>
        <div className={styles.filters}>
          <Segmented label="View" options={VIEW_FILTERS} value={view} onChange={setView} />
          <Segmented label="Status" options={STATUS_FILTERS} value={status} onChange={setStatus} />
          <label className={styles.from}>
            Check-in from
            <input type="date" value={from} onChange={(e) => setFrom(e.target.value)} />
          </label>
        </div>

        {view === 'archived' && <p className={styles.sub}>{ARCHIVE_NOTE}</p>}

        {bookings.isPending ? (
          <Skeleton height={320} radius={12} />
        ) : bookings.isError && !bookings.data ? (
          <ErrorState message={bookings.error.message} onRetry={() => bookings.refetch()} retrying={bookings.isFetching} />
        ) : bookings.data.bookings.length === 0 ? (
          <EmptyState compact title="No bookings match">
            Try another status or an earlier date.
          </EmptyState>
        ) : (
          <div className={styles.scroller}>
            <table className={cx(styles.table, bookings.isPlaceholderData && styles.stale)}>
              <thead>
                <tr>
                  <th scope="col">Ref</th>
                  <th scope="col">Guest</th>
                  <th scope="col">Stay</th>
                  <th scope="col">Dates</th>
                  <th scope="col" className={styles.num}>
                    Total
                  </th>
                  <th scope="col">Status</th>
                  <th scope="col">Booked</th>
                  <th scope="col">
                    <span className="sr-only">Calendar</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {bookings.data.bookings.map((b) => {
                  const s = STATUS_LABEL[b.status];
                  return (
                    <tr key={b.ref}>
                      <td className="mono">{b.ref}</td>
                      <td>
                        <strong>{b.guest.name}</strong>
                        <span className={styles.sub}>{formatParty(b.adults, b.children)}</span>
                      </td>
                      <td>
                        {b.room.name}
                        <span className={styles.sub}>{b.plan.name}</span>
                      </td>
                      <td>{formatStayShort(b.stay.check_in, b.stay.check_out)}</td>
                      <td className={`mono ${styles.num}`}>{formatINR(b.quote.total_paise)}</td>
                      <td>
                        <Pill tone={s.tone}>{s.label}</Pill>
                      </td>
                      <td className={styles.sub}>{formatTimestamp(b.created_at)}</td>
                      <td>
                        <button
                          type="button"
                          className={styles.ics}
                          aria-label={`Calendar file for ${b.ref}`}
                          onClick={() => {
                            const url = URL.createObjectURL(new Blob([stayCalendarFile(b)], { type: 'text/calendar' }));
                            Object.assign(document.createElement('a'), { href: url, download: `${b.ref}.ics` }).click();
                            URL.revokeObjectURL(url);
                          }}
                        >
                          .ics
                        </button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </Card>
    </div>
  );
}
