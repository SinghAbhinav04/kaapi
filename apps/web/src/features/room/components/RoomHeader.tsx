'use client';

import Link from 'next/link';
import { useRef, useState } from 'react';

import { CurrencySwitcher } from '@/features/currency';
import { searchToQuery, StaySearchForm, useStaySearch, type StaySearch } from '@/features/stay-search';
import { ROUTES } from '@/shared/const';
import { useDismiss } from '@/shared/hooks';
import { formatDay } from '@/shared/lib/dates';
import { formatParty } from '@/shared/lib/text';
import { Logo } from '@/shared/ui';

import styles from './RoomPage.module.css';

/** Logo, a way back to every stay, the searched stay with a "Change" panel, and My booking. Sticky, so the way out is never scrolled away. */
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
        <div className={styles.headerStart}>
          <Logo />
          <Link href={ROUTES.staysWith(searchToQuery(search))} className={styles.back}>
            <span aria-hidden="true">←</span> All stays
          </Link>
        </div>
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
        <div className={styles.headerEnd}>
          <CurrencySwitcher />
          <Link href={ROUTES.myBookings} className={styles.back}>
            My booking
          </Link>
        </div>
      </nav>
    </header>
  );
}
