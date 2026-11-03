'use client';

import { useState, type FormEvent } from 'react';

import { addDays, nightsBetween, todayIST } from '@/shared/lib/dates';
import { cx } from '@/shared/lib/cx';
import { plural } from '@/shared/lib/text';
import { Button } from '@/shared/ui';

import { MAX_DAYS_AHEAD, MAX_NIGHTS } from '../const';
import type { StaySearch } from '../types';
import { GuestsPicker } from './GuestsPicker';
import styles from './StaySearchForm.module.css';

interface StaySearchFormProps {
  initial: StaySearch;
  onSubmit: (search: StaySearch) => void;
  submitLabel?: string;
  /** hero: the glass bar on the home page. panel: inside the room page's "Change" popover. */
  variant?: 'hero' | 'panel';
}

/** Keeps its own draft until submit, so typing a date doesn't refetch prices on every keystroke. */
export function StaySearchForm({ initial, onSubmit, submitLabel = 'See prices for these dates', variant = 'hero' }: StaySearchFormProps) {
  const [draft, setDraft] = useState(initial);
  const today = todayIST();
  const nights = nightsBetween(draft.checkIn, draft.checkOut);
  const valid = nights >= 1 && nights <= MAX_NIGHTS && draft.checkIn >= today;

  const setCheckIn = (checkIn: string) => {
    if (!checkIn) return;
    // Keep the stay length when moving check-in, so a weekend stays a weekend.
    const keep = Math.max(1, nightsBetween(draft.checkIn, draft.checkOut));
    setDraft({ ...draft, checkIn, checkOut: addDays(checkIn, keep) });
  };

  const submit = (e: FormEvent) => {
    e.preventDefault();
    if (valid) onSubmit(draft);
  };

  return (
    <form aria-label="Check availability" className={cx(styles.form, styles[variant])} onSubmit={submit}>
      <label className={styles.cell}>
        <span className={styles.cellLabel}>CHECK-IN</span>
        <input
          type="date"
          className={styles.cellInput}
          value={draft.checkIn}
          min={today}
          max={addDays(today, MAX_DAYS_AHEAD)}
          onChange={(e) => setCheckIn(e.target.value)}
          required
        />
      </label>
      <label className={styles.cell}>
        <span className={styles.cellLabel}>CHECK-OUT{nights >= 1 ? ` · ${plural(nights, 'NIGHT')}` : ''}</span>
        <input
          type="date"
          className={styles.cellInput}
          value={draft.checkOut}
          min={addDays(draft.checkIn, 1)}
          max={addDays(draft.checkIn, MAX_NIGHTS)}
          onChange={(e) => e.target.value && setDraft({ ...draft, checkOut: e.target.value })}
          aria-invalid={!valid || undefined}
          required
        />
      </label>
      <GuestsPicker party={draft} onChange={(party) => setDraft({ ...draft, ...party })} />
      <Button type="submit" size="lg" className={styles.submit} disabled={!valid}>
        {submitLabel}
      </Button>
    </form>
  );
}
