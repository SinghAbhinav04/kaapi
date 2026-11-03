'use client';

import { useState, type ReactNode } from 'react';

import { useAnalytics, type AnalyticsBreakdown, type AnalyticsCount } from '@/apis/analytics';
import { Card, StaffPageHeader } from '@/features/staff-shell';
import { cx } from '@/shared/lib/cx';
import { formatINR } from '@/shared/lib/money';
import { plural } from '@/shared/lib/text';
import { ErrorState, Segmented, Skeleton } from '@/shared/ui';

import { BOOKING_STATUS_LABEL, CATEGORY_LABEL, CHANNEL_LABEL, COPY, DEFAULT_RANGE, RANGE_OPTIONS, type RangeValue } from '../const';
import { change, compactINR, percent, rangeParams, roundToRupee } from '../lib';
import styles from './Analytics.module.css';
import { BarList, OnTheBooks, RevenueColumns, TableView } from './Charts';

export function AnalyticsPage() {
  const [range, setRange] = useState<RangeValue>(DEFAULT_RANGE);
  const report = useAnalytics(rangeParams(Number(range)));

  const filters = <Segmented label="Booked in the last" options={RANGE_OPTIONS} value={range} onChange={setRange} />;

  if (report.isPending) {
    return (
      <div className={styles.page}>
        <StaffPageHeader title={COPY.title} description={COPY.description} actions={filters} />
        <Skeleton height={480} radius={20} />
      </div>
    );
  }
  if (report.isError && !report.data) {
    return <ErrorState message={report.error.message} onRetry={() => report.refetch()} retrying={report.isFetching} />;
  }

  const d = report.data;
  const k = d.kpis;
  const inr = (paise: number) => formatINR(roundToRupee(paise));
  const revenueChange = change(k.revenue_paise, k.previous_revenue_paise);

  return (
    <div className={styles.page}>
      <StaffPageHeader title={COPY.title} description={COPY.description} actions={filters} />

      <div className={cx(styles.page, report.isPlaceholderData && styles.stale)} aria-busy={report.isPlaceholderData}>
        <div className={styles.tiles}>
          <Tile label="Revenue (incl. GST)" value={inr(k.revenue_paise)}>
            {revenueChange === null ? (
              'No bookings in the period before to compare'
            ) : (
              <>
                <span className={styles.delta}>
                  {revenueChange >= 0 ? '▲' : '▼'} {percent(Math.abs(revenueChange))}
                </span>{' '}
                vs the {Number(range)} days before
              </>
            )}
          </Tile>
          <Tile label="Confirmed bookings" value={String(k.bookings)}>
            {plural(k.room_nights, 'room night')} · avg {inr(k.avg_booking_value_paise)}
          </Tile>
          <Tile label="Average daily rate" value={inr(k.adr_paise)}>
            Room revenue per room night, before GST
          </Tile>
          <Tile label="Commission saved" value={inr(k.commission_saved_paise)}>
            {COPY.commissionNote}
          </Tile>
          <Tile label="GST collected" value={inr(k.gst_paise)}>
            On {inr(k.room_revenue_paise)} of room revenue
          </Tile>
          <Tile label="Cancellation rate" value={percent(k.cancellation_rate, 1)}>
            Cancelled or refunded, of everything paid for
          </Tile>
        </div>

        <div className={styles.grid}>
          <ChartCard title={COPY.revenueTitle} note={`${d.range.from} to ${d.range.to}, IST. Hover or tab across for each day.`} wide>
            {k.bookings === 0 ? <p className={styles.empty}>{COPY.emptyRange}</p> : <RevenueColumns days={d.daily} />}
          </ChartCard>

          <ChartCard title="On the books" note={COPY.onTheBooksNote}>
            <OnTheBooks months={d.on_the_books} />
          </ChartCard>

          <ChartCard title="Revenue by room" note="Confirmed bookings made in this range.">
            <Breakdown rows={d.by_room} />
          </ChartCard>

          <ChartCard title="Revenue by rate plan" note="Which plans guests choose.">
            <Breakdown rows={d.by_plan} />
          </ChartCard>

          <ChartCard title={COPY.currencyTitle} note={COPY.currencyNote}>
            <Breakdown rows={d.by_display_currency} />
          </ChartCard>

          <ChartCard title="How far ahead guests book" note={`Average ${k.avg_lead_days.toFixed(0)} days; stays average ${k.avg_stay_nights.toFixed(1)} nights.`}>
            <Counts rows={d.lead_time} />
          </ChartCard>

          <ChartCard title="Booking outcomes" note="Every booking started in this range, paid or not.">
            <Counts rows={d.status_mix} labels={BOOKING_STATUS_LABEL} />
          </ChartCard>

          <ChartCard title="Enquiries" note={`${plural(d.enquiries.total, 'enquiry', 'enquiries')} received; ${d.enquiries.converted} became bookings.`}>
            <div className={styles.facts}>
              <div className={styles.fact}>
                <span>Median first reply</span>
                <strong>{d.enquiries.median_first_reply_minutes === null ? '—' : `${Math.round(d.enquiries.median_first_reply_minutes)} min`}</strong>
              </div>
              <div className={styles.fact}>
                <span>Read by AI (not keyword fallback)</span>
                <strong>{percent(d.enquiries.ai_handled_rate)}</strong>
              </div>
            </div>
            <Counts rows={d.enquiries.by_channel} labels={CHANNEL_LABEL} />
          </ChartCard>

          <ChartCard title="What enquiries are about" note="As the AI (or the keyword rules) classified them.">
            <Counts rows={d.enquiries.by_category} labels={CATEGORY_LABEL} />
          </ChartCard>
        </div>
      </div>
    </div>
  );
}

function Tile({ label, value, children }: { label: string; value: string; children: ReactNode }) {
  return (
    <Card className={styles.tile}>
      <span className={styles.tileLabel}>{label}</span>
      <p className={styles.tileValue}>{value}</p>
      <span className={styles.tileSub}>{children}</span>
    </Card>
  );
}

function ChartCard({ title, note, wide, children }: { title: string; note: string; wide?: boolean; children: ReactNode }) {
  return (
    <Card className={cx(styles.card, wide && styles.wide)}>
      <div className={styles.cardHead}>
        <h2>{title}</h2>
        <p>{note}</p>
      </div>
      {children}
    </Card>
  );
}

function Breakdown({ rows }: { rows: AnalyticsBreakdown[] }) {
  return (
    <>
      <BarList rows={rows.map((r) => ({ key: r.name, label: r.name, value: r.revenue_paise, display: compactINR(r.revenue_paise) }))} />
      <TableView head={['Name', 'Bookings', 'Room nights', 'Revenue']} rows={rows.map((r) => [r.name, r.bookings, r.room_nights, formatINR(r.revenue_paise)])} />
    </>
  );
}

function Counts({ rows, labels }: { rows: AnalyticsCount[]; labels?: Record<string, string> }) {
  if (rows.length === 0) return <p className={styles.empty}>Nothing in this range.</p>;
  return <BarList rows={rows.map((r) => ({ key: r.key, label: labels?.[r.key] ?? r.key, value: r.count, display: String(r.count) }))} />;
}
