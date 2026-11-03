'use client';

import Link from 'next/link';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import type { ReactNode } from 'react';

import { hasPermission, useLogout, type Permission, type User } from '@/apis/auth';
import { useDataChanges } from '@/apis/changes';
import { useInboxCounts } from '@/apis/enquiries';
import { ROUTES } from '@/shared/const';
import { cx } from '@/shared/lib/cx';
import { initials } from '@/shared/lib/text';
import { ButtonLink, EmptyState, ErrorState, Skeleton } from '@/shared/ui';

import { LANES, ROLE_TITLE, STAFF_NAV } from '../const';
import { useStaffGuard } from '../hooks';
import { NavIcon } from './NavIcon';
import styles from './StaffShell.module.css';

/** Permissions whose pages react to the live change stream. */
const WATCHED: readonly Permission[] = ['rates:read', 'analytics:read', 'promos:write', 'enquiries:read'];

export function StaffShell({ children }: { children: ReactNode }) {
  const { me, user, isStaff } = useStaffGuard();
  // One live stream per tab, for whoever looks at rates, analytics or offers.
  useDataChanges(!!user && isStaff && WATCHED.some((p) => hasPermission(user, p)));

  if (me.isError) {
    return (
      <div className={styles.center}>
        <ErrorState message={me.error.message} onRetry={() => me.refetch()} retrying={me.isFetching} />
      </div>
    );
  }
  if (!user) {
    return (
      <div className={styles.center}>
        <Skeleton height={240} width={360} radius={20} />
      </div>
    );
  }
  if (!isStaff) {
    return (
      <div className={styles.center}>
        <EmptyState title="This area is for resort staff" action={<ButtonLink href={ROUTES.myBookings}>Go to my bookings</ButtonLink>}>
          You&rsquo;re signed in as a guest.
        </EmptyState>
      </div>
    );
  }
  return (
    <div className={styles.shell}>
      <Sidebar user={user} />
      <div className={styles.content}>{children}</div>
    </div>
  );
}

function Sidebar({ user }: { user: User }) {
  const pathname = usePathname();
  const params = useSearchParams();
  const router = useRouter();
  const logout = useLogout();
  const canInbox = hasPermission(user, 'enquiries:read');
  const allLanes = hasPermission(user, 'enquiries:read_all_lanes');
  // Lane counts and the reply time from one small endpoint, refreshed by the live streams.
  const inbox = useInboxCounts(canInbox);
  const counts = new Map<string, number>((inbox.data?.lanes ?? []).map((l) => [l.lane, l.count]));
  const total = inbox.data ? [...counts.values()].reduce((a, b) => a + b, 0) : undefined;
  const activeLane = pathname.startsWith(ROUTES.staff.inbox) ? params.get('lane') : null;
  const replyTime = inbox.data?.reply_time ?? undefined;
  const median = replyTime?.median_minutes;
  const target = replyTime?.target_minutes ?? 30;

  return (
    <aside aria-label="Staff navigation" className={styles.sidebar}>
      <Link href={ROUTES.staff.inbox} className={styles.brand}>
        KAAPI ESTATE
      </Link>
      <nav className={styles.nav}>
        {STAFF_NAV.filter((item) => hasPermission(user, item.permission)).map((item) => {
          const active = pathname.startsWith(item.href);
          return (
            <div key={item.href}>
              <Link href={item.href} className={cx(styles.navItem, active && !activeLane && styles.navActive)} aria-current={active ? 'page' : undefined}>
                <NavIcon name={item.icon} active={active} />
                {item.label}
                {item.icon === 'inbox' && total !== undefined && <span className={cx(styles.count, styles.countStrong)}>{total}</span>}
              </Link>
              {item.icon === 'inbox' && canInbox && (
                <div className={styles.lanes}>
                  {LANES.filter((l) => allLanes || counts.has(l.lane)).map((l) => (
                    <Link
                      key={l.lane}
                      href={`${ROUTES.staff.inbox}?lane=${l.lane}`}
                      className={cx(styles.lane, activeLane === l.lane && styles.laneActive, l.lane === 'GM' && styles.laneAlert)}
                      aria-current={activeLane === l.lane ? 'true' : undefined}
                    >
                      <span className={cx(styles.dot, styles[`dot_${l.dot}`])} />
                      {l.label}
                      <span className={styles.count}>{counts.get(l.lane) ?? 0}</span>
                    </Link>
                  ))}
                </div>
              )}
            </div>
          );
        })}
      </nav>

      <div className={styles.grow} />
      {median !== undefined && (
        <div className={styles.metric}>
          <span className={styles.metricLabel}>Median first reply · this week</span>
          <span className={styles.metricValue}>{median === null ? '—' : `${Math.round(median)} min`}</span>
          <span className={styles.bar}>
            <span style={{ width: `${median === null ? 0 : Math.min(100, (median / target) * 100)}%` }} className={median !== null && median > target ? styles.barOver : undefined} />
          </span>
          <span className={median !== null && median > target ? styles.metricBad : styles.metricGood}>
            {median === null ? 'No replies yet this week' : median <= target ? `Within the ${target} min target` : `Over the ${target} min target`}
          </span>
        </div>
      )}
      <div className={styles.user}>
        <span className={styles.avatar}>{initials(user.name)}</span>
        <span className={styles.userText}>
          <strong>{user.name}</strong>
          <span>{ROLE_TITLE[user.role] ?? user.role}</span>
        </span>
        <button
          type="button"
          className={styles.signOut}
          onClick={() => logout.mutate(undefined, { onSettled: () => router.replace(ROUTES.staff.login) })}
          aria-label="Sign out"
          title="Sign out"
        >
          ⎋
        </button>
      </div>
    </aside>
  );
}
