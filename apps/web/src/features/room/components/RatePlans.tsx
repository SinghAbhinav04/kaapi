import type { Plan, PlanQuote } from '@/apis/catalog';
import { cancellationLine, inclusionsFor, nonRefundableSaving } from '@/features/booking-flow';
import type { IsoDate } from '@/shared/lib/dates';
import { formatINR } from '@/shared/lib/money';

import styles from './RoomPage.module.css';

interface RatePlansProps {
  plans: readonly PlanQuote[];
  catalogue: readonly Plan[];
  selected: string;
  checkIn: IsoDate;
  onSelect: (code: string) => void;
}

export function RatePlans({ plans, catalogue, selected, checkIn, onSelect }: RatePlansProps) {
  return (
    <section aria-labelledby="plans-title">
      <h2 id="plans-title" className={styles.sectionTitle}>
        Choose a rate
      </h2>
      <fieldset className={styles.plans}>
        <legend className="sr-only">Rate plan</legend>
        {plans.map((plan) => {
          const saving = nonRefundableSaving(plan, plans);
          const extras = inclusionsFor(plan.plan_id, catalogue).filter((x) => !/room/i.test(x));
          const detail = [extras.length ? `${extras.join(', ')} included` : null, saving ? `you save ${formatINR(saving)}` : null, cancellationLine(plan.refundable, checkIn)]
            .filter(Boolean)
            .join(' · ');
          return (
            <label key={plan.plan_id} className={styles.plan} data-selected={plan.code === selected}>
              <input type="radio" name="plan" value={plan.code} checked={plan.code === selected} onChange={() => onSelect(plan.code)} />
              <span className={styles.planText}>
                <strong>{plan.name}</strong>
                <span>{detail}</span>
              </span>
              <span className={styles.planPrice}>{formatINR(plan.quote.total_paise)}</span>
            </label>
          );
        })}
      </fieldset>
    </section>
  );
}
