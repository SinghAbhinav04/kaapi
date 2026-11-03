'use client';

import Link from 'next/link';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import type { ReactNode } from 'react';

import { hasPermission, useLogout, type User } from '@/apis/auth';
import { useInbox } from '@/apis/enquiries';
import { useDashboard } from '@/apis/operations';
import { ROUTES } from '@/shared/const';
import { cx } from '@/shared/lib/cx';
import { initials } from '@/shared/lib/text';
import { ButtonLink, EmptyState, ErrorState, Skeleton } from '@/shared/ui';

import { LANES, ROLE_TITLE, STAFF_NAV } from '../const';
import { useStaffGuard } from '../hooks';
import { NavIcon } from './NavIcon';
import styles from './StaffShell.module.css';

export function StaffShell({ children }: { children: ReactNode }) {
  const { me, user, isStaff } = useStaffGuard();

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
  // One shared, cached inbox query (the inbox page uses the same key, so no extra request).
  const inbox = useInbox({ limit: 300 });
  const dashboard = useDashboard(hasPermission(user, 'dashboard:read'));
  const counts = new Map<string, number>();
  for (const e of inbox.data?.enquiries ?? []) counts.set(e.lane, (counts.get(e.lane) ?? 0) + 1);
  const activeLane = pathname.startsWith(ROUTES.staff.inbox) ? params.get('lane') : null;
  const median = dashboard.data?.median_first_reply_minutes;
  const target = dashboard.data?.first_reply_target_minutes ?? 30;

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
                {item.icon === 'inbox' && inbox.data && <span className={cx(styles.count, styles.countStrong)}>{inbox.data.enquiries.length}</span>}
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
