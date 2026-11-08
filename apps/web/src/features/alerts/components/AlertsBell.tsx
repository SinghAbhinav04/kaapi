'use client';

import Link from 'next/link';
import { useEffect, useId, useRef, useState } from 'react';

import { useMarkAllNotificationsRead, useMarkNotificationRead, useNotifications } from '@/apis/notifications';
import { cx } from '@/shared/lib/cx';
import { formatAge } from '@/shared/lib/dates';

import { ALERT_LABEL, ALERTS_COPY } from '../const';
import { alertHref } from '../lib';
import styles from './AlertsBell.module.css';

/**
 * The console's alerts: complaints, enquiries the AI couldn't sort and late payments that need
 * a refund, for the signed-in person's team. New ones arrive through the change stream at once.
 */
export function AlertsBell() {
  const list = useNotifications();
  const read = useMarkNotificationRead();
  const readAll = useMarkAllNotificationsRead();
  const [open, setOpen] = useState(false);
  const panelId = useId();
  const root = useRef<HTMLDivElement>(null);
  const unread = list.data?.unread ?? 0;
  const items = list.data?.notifications ?? [];

  useEffect(() => {
    if (!open) return;
    const close = (e: MouseEvent | KeyboardEvent) => {
      if (e instanceof KeyboardEvent ? e.key === 'Escape' : !root.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener('mousedown', close);
    document.addEventListener('keydown', close);
    return () => {
      document.removeEventListener('mousedown', close);
      document.removeEventListener('keydown', close);
    };
  }, [open]);

  return (
    <div className={styles.root} ref={root}>
      <button
        type="button"
        className={cx(styles.bell, unread > 0 && styles.bellUnread)}
        aria-expanded={open}
        aria-controls={panelId}
        aria-label={unread > 0 ? `${ALERTS_COPY.button}, ${unread} unread` : ALERTS_COPY.button}
        onClick={() => setOpen((o) => !o)}
      >
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
          <path d="M6 8a6 6 0 0112 0c0 7 3 9 3 9H3s3-2 3-9" />
          <path d="M10.3 21a1.94 1.94 0 003.4 0" />
        </svg>
        {unread > 0 && <span className={styles.badge}>{unread > 99 ? '99+' : unread}</span>}
      </button>
      {open && (
        <div id={panelId} className={styles.panel} role="dialog" aria-label={ALERTS_COPY.title}>
          <div className={styles.head}>
            <strong>{ALERTS_COPY.title}</strong>
            {unread > 0 && (
              <button type="button" className={styles.markAll} onClick={() => readAll.mutate()} disabled={readAll.isPending}>
                {ALERTS_COPY.markAll}
              </button>
            )}
          </div>
          {items.length === 0 ? (
            <p className={styles.empty}>{ALERTS_COPY.empty}</p>
          ) : (
            <ul className={styles.list}>
              {items.map((n) => (
                <li key={n.id}>
                  <Link
                    href={alertHref(n)}
                    className={cx(styles.item, !n.read_at && styles.itemUnread)}
                    onClick={() => {
                      if (!n.read_at) read.mutate(n.id);
                      setOpen(false);
                    }}
                  >
                    <span className={cx(styles.kind, n.type === 'COMPLAINT_ALERT' && styles.kindAlert)}>
                      {ALERT_LABEL[n.type]} · {formatAge(n.created_at)}
                    </span>
                    <span className={styles.body}>{n.body}</span>
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </div>
  );
}
