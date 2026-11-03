'use client';

import { useRef, useState } from 'react';

import { StaySearchForm, useStaySearch, type StaySearch } from '@/features/stay-search';
import { useDismiss } from '@/shared/hooks';
import { formatDay } from '@/shared/lib/dates';
import { formatParty } from '@/shared/lib/text';
import { Logo } from '@/shared/ui';

import styles from './RoomPage.module.css';

/** Logo, the searched stay, and a "Change" panel that edits it in place. */
export function RoomHeader() {
  const { search, setSearch } = useStaySearch();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  useDismiss(ref, open, () => setOpen(false));

  const apply = (next: StaySearch) => {
    setSearch(next);
    setOpen(false);
  };

  return (
    <header className={styles.header}>
      <nav aria-label="Main" className={styles.headerInner}>
        <Logo />
        <div ref={ref} className={styles.stayPill}>
          <span className={styles.stayPart}>
            {formatDay(search.checkIn)} → {formatDay(search.checkOut)}
          </span>
          <span className={styles.stayDivider} aria-hidden="true" />
          <span className={styles.stayPart}>{formatParty(search.adults, search.children)}</span>
          <button type="button" className={styles.change} aria-expanded={open} onClick={() => setOpen((o) => !o)}>
            Change
          </button>
          {open && (
            <div className={styles.changePanel}>
              <StaySearchForm initial={search} onSubmit={apply} variant="panel" submitLabel="Update dates" />
            </div>
          )}
        </div>
      </nav>
    </header>
  );
}
