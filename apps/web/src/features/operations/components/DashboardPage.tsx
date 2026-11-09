'use client';

import Link from 'next/link';

import { useDashboard, type DashboardResponse } from '@/apis/operations';
import { Card, StaffPageHeader } from '@/features/staff-shell';
import { ROUTES } from '@/shared/const';
import { addDays, formatAge, formatDay, nightsBetween } from '@/shared/lib/dates';
import { formatINR } from '@/shared/lib/money';
import { formatParty, plural } from '@/shared/lib/text';
import { ErrorState, Pill, Skeleton } from '@/shared/ui';

import { LANE_NAME, TODAY_COPY as C, WEEK_DAYS } from '../const';
import styles from './Operations.module.css';

type Stay = DashboardResponse['stays'][number];

/** The desk's sheet: what needs a reply now, who comes and goes this week, and the week's goals. */
export function DashboardPage() {
  const dash = useDashboard();
  if (dash.isPending) return <Skeleton height={400} radius={20} />;
  if (dash.isError) return <ErrorState message={dash.error.message} onRetry={() => dash.refetch()} retrying={dash.isFetching} />;
  const d = dash.data;

  return (
    <div className={styles.page}>
      <StaffPageHeader title={C.title} description={C.description(formatDay(d.today))} />
      <div className={styles.tiles}>
        <Tile label={C.owed} value={d.overdue_count} tone={d.overdue_count ? 'bad' : 'good'} sub={C.owedSub(d.first_reply_target_minutes)} />
        <Tile label={C.complaints} value={d.open_complaints} tone={d.open_complaints ? 'bad' : 'good'} sub={C.complaintsSub} />
        <Tile label={C.refunds} value={d.refunds_pending} tone={d.refunds_pending ? 'bad' : undefined} sub={C.refundsSub} />
        <Tile label={C.tonight} value={d.sold_tonight} sub={C.tonightSub(d.allotment_tonight)} />
      </div>
      <Owed d={d} />
      <Week today={d.today} stays={d.stays} />
      <Goals d={d} />
    </div>
  );
}

function Tile({ label, value, sub, tone }: { label: string; value: number; sub: string; tone?: 'good' | 'bad' }) {
  return (
    <Card className={styles.tile}>
      <span className={styles.tileLabel}>{label}</span>
      <p className={styles.tileValue} data-tone={tone}>
        {value}
      </p>
      <span className={styles.tileSub}>{sub}</span>
    </Card>
  );
}

function Owed({ d }: { d: DashboardResponse }) {
  const more = d.overdue_count - d.overdue.length;
  return (
    <Card className={styles.card}>
      <h2 className={styles.sectionTitle}>{C.owedTitle}</h2>
      {d.overdue.length === 0 ? (
        <p className={styles.none}>{C.owedNone}</p>
      ) : (
        <ul className={styles.owed}>
          {d.overdue.map((w) => (
            <li key={w.ref}>
              <Link href={ROUTES.staff.enquiry(w.ref)} className={styles.owedRow}>
                <span className={styles.owedMeta}>
                  <span className="mono">{w.ref}</span>
                  {w.priority === 'HIGH' && <Pill tone="alert">{C.urgent}</Pill>}
                  {w.lane && <span>{LANE_NAME[w.lane] ?? w.lane}</span>}
                  <span className={styles.owedAge}>{C.waiting(formatAge(w.received_at))}</span>
                </span>
                <span className={styles.owedText}>{w.snippet}</span>
              </Link>
            </li>
          ))}
        </ul>
      )}
      {more > 0 && (
        <Link href={ROUTES.staff.inbox} className={styles.more}>
          {C.owedMore(more)}
        </Link>
      )}
    </Card>
  );
}

function Week({ today, stays }: { today: string; stays: Stay[] }) {
  const days = Array.from({ length: WEEK_DAYS }, (_, i) => addDays(today, i))
    .map((day) => ({ day, arriving: stays.filter((s) => s.check_in === day), leaving: stays.filter((s) => s.check_out === day) }))
    .filter((x) => x.arriving.length > 0 || x.leaving.length > 0);
  return (
    <Card className={styles.card}>
      <h2 className={styles.sectionTitle}>{C.weekTitle}</h2>
      {days.length === 0 ? (
        <p className={styles.none}>{C.weekNone}</p>
      ) : (
        <div className={styles.days}>
          {days.map(({ day, arriving, leaving }) => (
            <section key={day} className={styles.day}>
              <h3 className={styles.dayTitle}>{day === today ? `${C.todayLabel}, ${formatDay(day)}` : formatDay(day)}</h3>
              <StayList label={C.arriving} stays={arriving} />
              <StayList label={C.leaving} stays={leaving} />
            </section>
          ))}
        </div>
      )}
    </Card>
  );
}

function StayList({ label, stays }: { label: string; stays: Stay[] }) {
  if (stays.length === 0) return null;
  return (
    <div>
      <span className={styles.tileLabel}>
        {label} · {stays.length}
      </span>
      <ul className={styles.stays}>
        {stays.map((s) => (
          <li key={s.ref}>
            <strong>{s.guest_name}</strong>
            <span>
              {s.room_type}
              {s.rooms > 1 ? ` × ${s.rooms}` : ''} · {plural(nightsBetween(s.check_in, s.check_out), 'night')} · {formatParty(s.adults, s.children)}
            </span>
            <span className="mono">{s.ref}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}

function Goals({ d }: { d: DashboardResponse }) {
  const median = d.median_first_reply_minutes;
  const onTarget = median !== null && median <= d.first_reply_target_minutes;
  return (
    <Card className={styles.card}>
      <h2 className={styles.sectionTitle}>{C.goalsTitle}</h2>
      <dl className={styles.goals}>
        <div>
          <dt>{C.medianReply}</dt>
          <dd data-tone={median === null ? undefined : onTarget ? 'good' : 'bad'}>{median === null ? '—' : `${Math.round(median)} min`}</dd>
          <dd className={styles.tileSub}>{C.medianReplySub(d.first_reply_target_minutes, d.replied_this_week)}</dd>
        </div>
        <div>
          <dt>{C.direct}</dt>
          <dd>{d.direct_bookings_this_week}</dd>
          <dd className={styles.tileSub}>{C.directSub(formatINR(d.direct_revenue_this_week_paise))}</dd>
        </div>
        <div>
          <dt>{C.overbookings}</dt>
          <dd data-tone={d.overbookings === 0 ? 'good' : 'bad'}>{d.overbookings}</dd>
          <dd className={styles.tileSub}>{C.overbookingsSub}</dd>
        </div>
        <div>
          <dt>{C.aiCorrected}</dt>
          <dd>{d.ai_correction_rate === null ? '—' : `${Math.round(d.ai_correction_rate * 100)}%`}</dd>
          <dd className={styles.tileSub}>{d.ai_enabled ? C.aiWaiting(d.awaiting_ai) : C.aiOff}</dd>
        </div>
      </dl>
    </Card>
  );
}
