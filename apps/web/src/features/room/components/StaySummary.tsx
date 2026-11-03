'use client';

import { useState, type FormEvent } from 'react';

import type { AvailabilityOption, PlanQuote } from '@/apis/catalog';
import type { ApiError } from '@/apis/http';
import { ApproxPrice } from '@/features/currency';
import type { StaySearch } from '@/features/stay-search';
import { formatStay, formatWeekdayDay, nightsBetween } from '@/shared/lib/dates';
import { formatDiscount, formatINR, formatRate } from '@/shared/lib/money';
import { formatParty, plural } from '@/shared/lib/text';
import { Button, Pill, Skeleton } from '@/shared/ui';

import { PROMO_PATTERN, SCARCITY_THRESHOLD } from '../const';
import { gstSummary } from '../lib';
import styles from './RoomPage.module.css';

interface StaySummaryProps {
  roomName: string;
  search: StaySearch;
  option: AvailabilityOption | undefined;
  plan: PlanQuote | undefined;
  loading: boolean;
  updating: boolean;
  promo: string | undefined;
  onPromo: (code: string | undefined) => void;
  onReserve: () => void;
  reserving: boolean;
  error: ApiError | null;
}

export function StaySummary(props: StaySummaryProps) {
  const { roomName, search, option, plan, loading, updating, promo, onPromo, onReserve, reserving, error } = props;
  const nights = nightsBetween(search.checkIn, search.checkOut);
  const quote = plan?.quote;
  const available = option?.status === 'AVAILABLE';

  return (
    <aside aria-label="Your stay" className={styles.summary} aria-busy={updating}>
      <div className={styles.summaryHead}>
        <span className="label">YOUR STAY</span>
        {available && option.left <= SCARCITY_THRESHOLD && (
          <Pill tone="paper" mono>
            {option.left} LEFT ON OUR SITE
          </Pill>
        )}
      </div>
      <div>
        <p className={styles.summaryTitle}>
          {roomName}
          {plan ? ` · ${plan.name}` : ''}
        </p>
        <p className={styles.muted}>
          {formatStay(search.checkIn, search.checkOut)} · {plural(nights, 'night')} · {formatParty(search.adults, search.children)}
        </p>
      </div>

      {loading ? (
        <div className={styles.summaryLoading}>
          <Skeleton height={18} />
          <Skeleton height={18} />
          <Skeleton height={32} width="60%" />
        </div>
      ) : !available ? (
        <p className={styles.unavailable} role="status">
          {option?.reason ?? 'Not available for these dates.'} Pick other dates in the calendar.
        </p>
      ) : (
        quote && (
          <>
            <table className={styles.nights}>
              <caption className="label">NIGHT BY NIGHT</caption>
              <tbody>
                {quote.lines.map((line) => (
                  <tr key={line.date}>
                    <th scope="row">{formatWeekdayDay(line.date)}</th>
                    <td className="mono">{formatINR(line.room_paise)}</td>
                    <td className={`mono ${styles.muted}`}>
                      +{formatRate(line.gst_rate_bps)} {formatINR(line.gst_paise)}
                    </td>
                  </tr>
                ))}
                {quote.discount_paise > 0 && (
                  <tr className={styles.discount}>
                    <th scope="row">{quote.promo_code ?? 'Discount'}</th>
                    <td className="mono">{formatDiscount(quote.discount_paise)}</td>
                    <td />
                  </tr>
                )}
              </tbody>
            </table>
            <div className={`${styles.total} ${updating ? styles.stale : ''}`}>
              <span>Total, incl. GST</span>
              <span className={styles.totalAmount}>{formatINR(quote.total_paise)}</span>
            </div>
            <ApproxPrice paise={quote.total_paise} withNote />
            <p className={styles.note}>
              {gstSummary(quote.lines)}. GST is 5% on nights up to ₹7,500 and 18% above, after any discount.
            </p>
          </>
        )
      )}

      <PromoField promo={promo} onPromo={onPromo} error={option?.promo_error ?? null} applied={!!quote?.promo_code} />

      {error && (
        <p className={styles.reserveError} role="alert">
          {error.code === 'SOLD_OUT' ? 'Someone just booked the last room for these dates. Prices and rooms left have been refreshed.' : error.message}
        </p>
      )}
      <Button size="lg" block onClick={onReserve} loading={reserving} disabled={!available || !plan}>
        Reserve · we hold it for 10 min
      </Button>
      <p className={styles.center}>Step 1 of 3 · No payment yet</p>
    </aside>
  );
}

function PromoField({ promo, onPromo, error, applied }: { promo?: string; onPromo: (c?: string) => void; error: string | null; applied: boolean }) {
  const [open, setOpen] = useState(!!promo);
  const [value, setValue] = useState(promo ?? '');
  const submit = (e: FormEvent) => {
    e.preventDefault();
    const code = value.trim().toUpperCase();
    onPromo(PROMO_PATTERN.test(code) ? code : undefined);
  };
  if (!open) {
    return (
      <button type="button" className={styles.linkButton} onClick={() => setOpen(true)}>
        Have a promo code?
      </button>
    );
  }
  return (
    <form className={styles.promo} onSubmit={submit}>
      <label className={styles.promoField}>
        <span className="sr-only">Promo code</span>
        <input value={value} onChange={(e) => setValue(e.target.value)} placeholder="PROMO CODE" autoCapitalize="characters" spellCheck={false} />
      </label>
      {promo ? (
        <Button
          variant="outline"
          onClick={() => {
            setValue('');
            onPromo(undefined);
          }}
        >
          Remove
        </Button>
      ) : (
        <Button type="submit" variant="outline">
          Apply
        </Button>
      )}
      {promo && (
        <p className={error ? styles.promoError : styles.promoOk} role="status">
          {error ?? (applied ? `${promo} applied.` : 'Checking…')}
        </p>
      )}
    </form>
  );
}
