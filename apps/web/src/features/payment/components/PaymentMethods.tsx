'use client';

import QRCode from 'qrcode';
import { useEffect, useEffectEvent, useState } from 'react';

import { formatINR } from '@/shared/lib/money';
import { Button, Segmented, SelectField, TextField } from '@/shared/ui';

import { BANKS, METHODS, QR_AUTO_PAY_SECONDS, UPI_PAYEE, WALLETS, type Method } from '../const';
import styles from './PaymentPage.module.css';

/**
 * The ways to pay, as a real gateway shows them. Test mode: every option ends in the same
 * signed test payment, and nothing typed here (card numbers included) leaves the browser.
 */
export function PaymentMethods({ amountPaise, bookingRef, busy, onPay }: { amountPaise: number; bookingRef: string; busy: boolean; onPay: () => void }) {
  const [method, setMethod] = useState<Method>('UPI');
  return (
    <div className={styles.methods}>
      <Segmented label="Pay with" options={METHODS} value={method} onChange={setMethod} />
      {method === 'UPI' && <Upi amountPaise={amountPaise} bookingRef={bookingRef} busy={busy} onPay={onPay} />}
      {method === 'CARD' && <Card amountPaise={amountPaise} busy={busy} onPay={onPay} />}
      {method === 'NETBANKING' && <Choice label="Your bank" options={BANKS} amountPaise={amountPaise} busy={busy} onPay={onPay} />}
      {method === 'WALLET' && <Choice label="Wallet" options={WALLETS} amountPaise={amountPaise} busy={busy} onPay={onPay} />}
    </div>
  );
}

function Upi({ amountPaise, bookingRef, busy, onPay }: { amountPaise: number; bookingRef: string; busy: boolean; onPay: () => void }) {
  const [qr, setQr] = useState<string | null>(null);
  const [left, setLeft] = useState(QR_AUTO_PAY_SECONDS);
  const [vpa, setVpa] = useState('');
  const link =
    `upi://pay?pa=${encodeURIComponent(UPI_PAYEE.vpa)}&pn=${encodeURIComponent(UPI_PAYEE.name)}` +
    `&am=${(amountPaise / 100).toFixed(2)}&cu=INR&tn=${encodeURIComponent(`Booking ${bookingRef}`)}`;

  useEffect(() => {
    let live = true;
    QRCode.toDataURL(link, { margin: 1, width: 220, color: { dark: '#141210', light: '#fffdf9' } }).then((url) => live && setQr(url));
    return () => {
      live = false;
    };
  }, [link]);

  // Test mode stands in for the bank: a few seconds after the code shows, the "scan" succeeds,
  // once. Any other payment attempt (a decline, the UPI ID) ends the countdown for good, and a
  // failed attempt is never retried by itself.
  const [stopped, setStopped] = useState(false);
  const paid = useEffectEvent(() => onPay());
  useEffect(() => {
    if (!qr || stopped) return;
    const t = window.setTimeout(() => {
      if (busy) setStopped(true);
      else if (left <= 1) {
        setStopped(true);
        paid();
      } else setLeft((s) => s - 1);
    }, 1000);
    return () => window.clearTimeout(t);
  }, [qr, left, busy, stopped]);

  return (
    <div className={styles.upi}>
      <div className={styles.qr}>
        {/* eslint-disable-next-line @next/next/no-img-element -- a generated data: URL; next/image can't optimise it */}
        {qr ? <img src={qr} width={220} height={220} alt={`UPI QR code to pay ${formatINR(amountPaise)} to ${UPI_PAYEE.name}`} /> : <span className={styles.qrWait} />}
      </div>
      <p className={styles.detail}>
        Scan with any UPI app (GPay, PhonePe, Paytm, BHIM).
        <br />
        <span className="mono">{busy ? 'Confirming your payment…' : stopped ? 'Test mode: pay with your UPI ID or another way.' : `Test mode: the scan completes in ${left}s.`}</span>
      </p>
      <div className={styles.inline}>
        <TextField label="Or pay with your UPI ID" placeholder="name@bank" value={vpa} onChange={(e) => setVpa(e.target.value)} />
        <Button onClick={onPay} loading={busy} disabled={!vpa.includes('@')}>
          Request
        </Button>
      </div>
    </div>
  );
}

function Card({ amountPaise, busy, onPay }: { amountPaise: number; busy: boolean; onPay: () => void }) {
  const [number, setNumber] = useState('');
  const [expiry, setExpiry] = useState('');
  const [cvv, setCvv] = useState('');
  const [name, setName] = useState('');
  const digits = number.replace(/\D/g, '');
  const ready = digits.length >= 12 && /^\d{2}\/\d{2}$/.test(expiry) && /^\d{3,4}$/.test(cvv) && name.trim().length > 1;
  return (
    <div className={styles.form}>
      <TextField
        label="Card number"
        inputMode="numeric"
        autoComplete="off"
        placeholder="4111 1111 1111 1111"
        value={number}
        onChange={(e) => setNumber(e.target.value.replace(/[^\d ]/g, '').slice(0, 23))}
        hint="Test mode: any number works and nothing you type leaves this page."
      />
      <div className={styles.inline}>
        <TextField label="Expiry" placeholder="MM/YY" inputMode="numeric" autoComplete="off" value={expiry} onChange={(e) => setExpiry(e.target.value.slice(0, 5))} />
        <TextField label="CVV" inputMode="numeric" autoComplete="off" type="password" value={cvv} onChange={(e) => setCvv(e.target.value.replace(/\D/g, '').slice(0, 4))} />
      </div>
      <TextField label="Name on card" autoComplete="off" value={name} onChange={(e) => setName(e.target.value)} />
      <Button size="lg" block onClick={onPay} loading={busy} disabled={!ready}>
        Pay {formatINR(amountPaise)}
      </Button>
    </div>
  );
}

function Choice({
  label,
  options,
  amountPaise,
  busy,
  onPay,
}: {
  label: string;
  options: readonly { value: string; label: string }[];
  amountPaise: number;
  busy: boolean;
  onPay: () => void;
}) {
  const [value, setValue] = useState(options[0]?.value ?? '');
  return (
    <div className={styles.form}>
      <SelectField label={label} value={value} onChange={(e) => setValue(e.target.value)} options={options} />
      <Button size="lg" block onClick={onPay} loading={busy}>
        Pay {formatINR(amountPaise)}
      </Button>
    </div>
  );
}
