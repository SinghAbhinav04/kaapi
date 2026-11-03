import Image from 'next/image';

import { cancellationLine, type HoldTicket } from '@/features/booking-flow';
import { ApproxPrice } from '@/features/currency';
import { formatStay } from '@/shared/lib/dates';
import { formatDiscount, formatINR } from '@/shared/lib/money';
import { formatParty, plural } from '@/shared/lib/text';

import styles from './CheckoutPage.module.css';

/** The frozen quote from the hold: exactly what the guest will pay. */
export function OrderSummary({ ticket }: { ticket: HoldTicket }) {
  const { hold } = ticket;
  const quote = hold.quote;
  const rates = [...new Set(quote.lines.map((l) => l.gst_rate_bps / 100))];
  const gstLabel = rates.length === 1 ? `GST at ${rates[0]}%` : 'GST at 5% and 18%, by night';
  return (
    <aside aria-label="Summary" className={styles.summary}>
      {ticket.photo && (
        <div className={styles.summaryPhoto}>
          <Image src={ticket.photo.url} alt={ticket.photo.alt} fill sizes="380px" />
        </div>
      )}
      <div>
        <p className={styles.summaryTitle}>
          {ticket.roomName} · {ticket.planName}
        </p>
        <p className={styles.muted}>
          {formatStay(hold.stay.check_in, hold.stay.check_out)} · {plural(hold.stay.nights, 'night')} · {formatParty(ticket.adults, ticket.children)}
        </p>
        <p className={ticket.refundable ? styles.good : styles.muted}>{cancellationLine(ticket.refundable, hold.stay.check_in)}</p>
      </div>
      <dl className={styles.lines}>
        <dt>Room, {plural(hold.stay.nights, 'night')}</dt>
        <dd className="mono">{formatINR(quote.subtotal_paise)}</dd>
        {quote.discount_paise > 0 && (
          <>
            <dt className={styles.good}>{quote.promo_code ?? 'Discount'}</dt>
            <dd className={`mono ${styles.good}`}>{formatDiscount(quote.discount_paise)}</dd>
          </>
        )}
        <dt>{gstLabel}</dt>
        <dd className="mono">{formatINR(quote.tax_paise)}</dd>
      </dl>
      <div className={styles.youPay}>
        <span>You pay</span>
        <span className={styles.youPayAmount}>{formatINR(quote.total_paise)}</span>
      </div>
      <ApproxPrice paise={quote.total_paise} withNote />
    </aside>
  );
}
