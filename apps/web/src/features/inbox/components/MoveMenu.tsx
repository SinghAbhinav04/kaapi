'use client';

import { useRef, useState } from 'react';

import { useRouteEnquiry, type Category, type Enquiry } from '@/apis/enquiries';
import { useDismiss } from '@/shared/hooks';

import { CATEGORY_LABEL, CATEGORY_LANE, LANE_LABEL } from '../const';
import styles from './Inbox.module.css';

const CATEGORIES: readonly Category[] = ['ROOM', 'WEDDING', 'OFFSITE', 'COMPLAINT', 'OTHER'];

/**
 * One-click re-route. Picking a type moves the enquiry to that type's team and writes a
 * fresh draft; the row leaves this lane immediately (optimistic) and the change is logged.
 */
export function MoveMenu({ enquiry, onMoved }: { enquiry: Enquiry; onMoved: () => void }) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const route = useRouteEnquiry(enquiry.ref);
  useDismiss(ref, open, () => setOpen(false));

  const move = (category: Category) => {
    setOpen(false);
    route.mutate({ lane: CATEGORY_LANE[category], category }, { onSuccess: onMoved });
  };

  return (
    <div ref={ref} className={styles.menuWrap}>
      <button type="button" className={styles.menuButton} aria-expanded={open} aria-haspopup="menu" onClick={() => setOpen((o) => !o)} disabled={route.isPending}>
        {route.isPending ? 'Moving…' : 'Move ▾'}
      </button>
      {open && (
        <div role="menu" className={styles.menu}>
          <p className={styles.menuTitle}>It&rsquo;s really a…</p>
          {CATEGORIES.map((c) => (
            <button key={c} role="menuitem" type="button" className={styles.menuItem} disabled={c === enquiry.category} onClick={() => move(c)}>
              {CATEGORY_LABEL[c]}
              <span className={styles.menuHint}>→ {LANE_LABEL[CATEGORY_LANE[c]]}</span>
            </button>
          ))}
        </div>
      )}
      {route.isError && (
        <p className={styles.error} role="alert">
          {route.error.message}
        </p>
      )}
    </div>
  );
}
