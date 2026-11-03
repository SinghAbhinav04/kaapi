const PAISE_PER_RUPEE = 100;

const inr = new Intl.NumberFormat('en-IN', { style: 'currency', currency: 'INR', maximumFractionDigits: 0 });
const inrExact = new Intl.NumberFormat('en-IN', { style: 'currency', currency: 'INR', minimumFractionDigits: 2 });
const oneDecimal = new Intl.NumberFormat('en-IN', { minimumFractionDigits: 1, maximumFractionDigits: 1 });

/** ₹44,368 — Indian digit grouping; paise shown only when there are any. */
export function formatINR(paise: number): string {
  return paise % PAISE_PER_RUPEE === 0 ? inr.format(paise / PAISE_PER_RUPEE) : inrExact.format(paise / PAISE_PER_RUPEE);
}

/** −₹4,000 for discounts. */
export function formatDiscount(paise: number): string {
  return `−${formatINR(Math.abs(paise))}`;
}

/** 17.5 — thousands of rupees, for dense calendars and grids. */
export function formatThousands(paise: number): string {
  return oneDecimal.format(paise / PAISE_PER_RUPEE / 1000);
}

export function rupeesToPaise(rupees: number): number {
  return Math.round(rupees * PAISE_PER_RUPEE);
}

export function paiseToRupees(paise: number): number {
  return paise / PAISE_PER_RUPEE;
}

/** GST rate from basis points: 1800 → "18%". */
export function formatRate(bps: number): string {
  return `${bps / 100}%`;
}
