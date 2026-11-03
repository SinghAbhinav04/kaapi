'use client';

import { useState, type FormEvent } from 'react';

import { useCorrectEnquiry, type Enquiry } from '@/apis/enquiries';
import { formatDayMonth, formatWeekdayDay } from '@/shared/lib/dates';
import { formatINR } from '@/shared/lib/money';
import { plural } from '@/shared/lib/text';
import { Button, TextField } from '@/shared/ui';

import { CATEGORY_LABEL, LANE_LABEL } from '../const';
import { confidencePercent } from '../lib';
import styles from './Inbox.module.css';

export function AiUnderstood({ enquiry, canEdit }: { enquiry: Enquiry; canEdit: boolean }) {
  const [editing, setEditing] = useState(false);
  const x = enquiry.extracted;
  const pct = confidencePercent(enquiry.confidence);
  const isEvent = enquiry.category === 'WEDDING' || enquiry.category === 'OFFSITE';

  return (
    <section aria-labelledby="ai-title" className={styles.block}>
      <div className={styles.blockHead}>
        <h3 id="ai-title" className={styles.blockTitle}>
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--estate)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <path d="M12 3l1.9 4.6L18.5 9l-4.6 1.9L12 15.5l-1.9-4.6L5.5 9l4.6-1.4z" />
          </svg>
          What the AI understood
        </h3>
        {canEdit && !editing && enquiry.ai_status !== 'PENDING' && (
          <button type="button" className={styles.textButton} onClick={() => setEditing(true)}>
            Correct
          </button>
        )}
      </div>

      {enquiry.ai_status === 'PENDING' ? (
        <p className={styles.muted} role="status">
          The AI is reading this message. Results appear here in a few seconds.
        </p>
      ) : editing ? (
        <CorrectForm enquiry={enquiry} onDone={() => setEditing(false)} />
      ) : (
        <>
          <div className={styles.tiles}>
            <div className={styles.tile}>
              <span className={styles.tileLabel}>Type</span>
              <p className={styles.tileValue}>{enquiry.category ? CATEGORY_LABEL[enquiry.category] : '—'}</p>
              {pct !== null && (
                <span className={styles.confidence}>
                  <span className={styles.track}>
                    <span style={{ width: `${pct}%` }} className={pct < 60 ? styles.trackLow : undefined} />
                  </span>
                  <span className={pct < 60 ? styles.pctLow : styles.pct}>{pct}%</span>
                </span>
              )}
            </div>
            {isEvent ? (
              <>
                <div className={styles.tile}>
                  <span className={styles.tileLabel}>When</span>
                  <p className={styles.tileValue}>{x.event_month ?? (x.check_in ? formatDayMonth(x.check_in) : 'Not given')}</p>
                  {x.nights && <span className={styles.tileSub}>{plural(x.nights, 'night')}</span>}
                </div>
                <div className={styles.tile}>
                  <span className={styles.tileLabel}>Guests</span>
                  <p className={styles.tileValue}>{x.guest_count ?? 'Not given'}</p>
                  {x.budget_paise && (
                    <span className={styles.tileSub}>
                      Budget {formatINR(x.budget_paise)}
                      {x.budget_basis === 'PER_NIGHT' ? ' a night' : ''}
                    </span>
                  )}
                </div>
              </>
            ) : (
              <>
                <div className={styles.tile}>
                  <span className={styles.tileLabel}>Dates</span>
                  <p className={styles.tileValue}>{x.check_in && x.check_out ? `${formatDayMonth(x.check_in)} – ${formatDayMonth(x.check_out)}` : 'Not given'}</p>
                  {x.check_in && x.check_out && x.nights && (
                    <span className={styles.tileSub}>
                      {formatWeekdayDay(x.check_in)} to {formatWeekdayDay(x.check_out)} · {plural(x.nights, 'night')}
                    </span>
                  )}
                </div>
                <div className={styles.tile}>
                  <span className={styles.tileLabel}>Guests</span>
                  <p className={styles.tileValue}>{x.adults ? `${x.adults}${x.children ? ` + ${plural(x.children, 'child', 'children')}` : ''}` : 'Not given'}</p>
                  {x.rooms && x.rooms > 1 && <span className={styles.tileSub}>{plural(x.rooms, 'room')}</span>}
                </div>
              </>
            )}
            <div className={styles.tile}>
              <span className={styles.tileLabel}>Owner</span>
              <p className={styles.tileValue}>{LANE_LABEL[enquiry.lane]}</p>
              <span className={styles.tileSub}>{enquiry.assigned_to?.name ?? 'Unassigned'}</span>
            </div>
          </div>
          <p className={styles.reason}>
            {enquiry.ai_status === 'FALLBACK' ? 'Keyword rules only: ' : ''}
            {enquiry.ai_reason}
            {enquiry.ai_model ? ` · ${enquiry.ai_model.replace(/^\w+:/, '')}` : ''}
            {enquiry.summary ? ` · “${enquiry.summary}”` : ''}
          </p>
        </>
      )}
    </section>
  );
}

function CorrectForm({ enquiry, onDone }: { enquiry: Enquiry; onDone: () => void }) {
  const x = enquiry.extracted;
  const [checkIn, setCheckIn] = useState(x.check_in ?? '');
  const [checkOut, setCheckOut] = useState(x.check_out ?? '');
  const [adults, setAdults] = useState(x.adults?.toString() ?? '');
  const [children, setChildren] = useState(x.children?.toString() ?? '');
  const [guests, setGuests] = useState(x.guest_count?.toString() ?? '');
  const correct = useCorrectEnquiry(enquiry.ref);
  const num = (s: string) => (s.trim() === '' ? undefined : Number(s));

  const submit = (e: FormEvent) => {
    e.preventDefault();
    correct.mutate(
      {
        check_in: checkIn || undefined,
        check_out: checkOut || undefined,
        adults: num(adults),
        children: num(children),
        guest_count: num(guests),
      },
      { onSuccess: onDone },
    );
  };

  return (
    <form className={styles.correct} onSubmit={submit}>
      <div className={styles.correctGrid}>
        <TextField label="Check-in" type="date" value={checkIn} onChange={(e) => setCheckIn(e.target.value)} />
        <TextField label="Check-out" type="date" value={checkOut} onChange={(e) => setCheckOut(e.target.value)} />
        <TextField label="Adults" inputMode="numeric" value={adults} onChange={(e) => setAdults(e.target.value)} />
        <TextField label="Children" inputMode="numeric" value={children} onChange={(e) => setChildren(e.target.value)} />
        <TextField label="Event guests" inputMode="numeric" value={guests} onChange={(e) => setGuests(e.target.value)} />
      </div>
      <p className={styles.muted}>Saving re-prices the draft&rsquo;s quote from the rate table.</p>
      {correct.isError && (
        <p className={styles.error} role="alert">
          {Object.values(correct.error.fieldErrors)[0] ?? correct.error.message}
        </p>
      )}
      <div className={styles.row}>
        <Button variant="quiet" onClick={onDone}>
          Cancel
        </Button>
        <Button type="submit" loading={correct.isPending}>
          Save corrections
        </Button>
      </div>
    </form>
  );
}
