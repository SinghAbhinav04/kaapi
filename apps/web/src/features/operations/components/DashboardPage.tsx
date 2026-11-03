'use client';

import { useDashboard } from '@/apis/operations';
import { Card, StaffPageHeader } from '@/features/staff-shell';
import { formatINR } from '@/shared/lib/money';
import { ErrorState, Skeleton } from '@/shared/ui';

import { LANE_NAME } from '../const';
import styles from './Operations.module.css';

/** The brief's success measures, live. */
export function DashboardPage() {
  const dash = useDashboard();
  if (dash.isPending) return <Skeleton height={400} radius={20} />;
  if (dash.isError) return <ErrorState message={dash.error.message} onRetry={() => dash.refetch()} retrying={dash.isFetching} />;
  const d = dash.data;
  const median = d.median_first_reply_minutes;
  const onTarget = median !== null && median <= d.first_reply_target_minutes;

  return (
    <div className={styles.page}>
      <StaffPageHeader title="Dashboard" description="This week, against the goals: replies within 30 minutes, more direct bookings, zero overbookings." />
      <div className={styles.tiles}>
        <Card className={styles.tile}>
          <span className={styles.tileLabel}>Median first reply</span>
          <p className={styles.tileValue} data-tone={median === null ? undefined : onTarget ? 'good' : 'bad'}>
            {median === null ? '—' : `${Math.round(median)} min`}
          </p>
          <span className={styles.tileSub}>
            Target {d.first_reply_target_minutes} min · {d.replied_this_week} replied this week
          </span>
        </Card>
        <Card className={styles.tile}>
          <span className={styles.tileLabel}>Direct bookings this week</span>
          <p className={styles.tileValue}>{d.direct_bookings_this_week}</p>
          <span className={styles.tileSub}>{formatINR(d.direct_revenue_this_week_paise)} with no agency commission</span>
        </Card>
        <Card className={styles.tile}>
          <span className={styles.tileLabel}>Overbookings</span>
          <p className={styles.tileValue} data-tone={d.overbookings === 0 ? 'good' : 'bad'}>
            {d.overbookings}
          </p>
          <span className={styles.tileSub}>The database refuses to sell a room twice</span>
        </Card>
        <Card className={styles.tile}>
          <span className={styles.tileLabel}>Open complaints</span>
          <p className={styles.tileValue} data-tone={d.open_complaints ? 'bad' : 'good'}>
            {d.open_complaints}
          </p>
          <span className={styles.tileSub}>Each one alerted the GM on arrival</span>
        </Card>
        <Card className={styles.tile}>
          <span className={styles.tileLabel}>AI routing corrected by staff</span>
          <p className={styles.tileValue}>{d.ai_correction_rate === null ? '—' : `${Math.round(d.ai_correction_rate * 100)}%`}</p>
          <span className={styles.tileSub}>{d.ai_enabled ? 'Share of AI routings changed this week' : 'AI is off: keyword rules only'}</span>
        </Card>
        <Card className={styles.tile}>
          <span className={styles.tileLabel}>Waiting for the AI</span>
          <p className={styles.tileValue}>{d.awaiting_ai}</p>
          <span className={styles.tileSub}>Read within seconds, paced to the model&rsquo;s free-tier limits</span>
        </Card>
      </div>
      <Card className={styles.card}>
        <h2 className={styles.sectionTitle}>Open enquiries by lane</h2>
        <ul className={styles.lanes}>
          {Object.entries(d.open_by_lane).map(([lane, count]) => (
            <li key={lane}>
              <span>{LANE_NAME[lane] ?? lane}</span>
              <strong>{count}</strong>
            </li>
          ))}
        </ul>
      </Card>
    </div>
  );
}
