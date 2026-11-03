import type { Plan, PlanQuote } from '@/apis/catalog';
import { addDays, formatDay, type IsoDate } from '@/shared/lib/dates';

import { CANCELLATION_DAYS } from './const';

/** Last day a flexible booking can be cancelled for free. */
export const freeCancellationUntil = (checkIn: IsoDate) => addDays(checkIn, -CANCELLATION_DAYS);

/** "Free cancellation until Sat 7 Nov" or "Non-refundable", in the guest's words. */
export function cancellationLine(refundable: boolean, checkIn: IsoDate): string {
  return refundable ? `Free cancellation until ${formatDay(freeCancellationUntil(checkIn))}` : 'Pay now, no changes or refunds';
}

/** How much the non-refundable rate saves against its flexible twin, if there is one. */
export function nonRefundableSaving(plan: PlanQuote, all: readonly PlanQuote[]): number | null {
  if (plan.refundable) return null;
  const flexible = all.find((p) => p.refundable && p.code.split('-')[0] === plan.code.split('-')[0]);
  return flexible ? flexible.quote.total_paise - plan.quote.total_paise : null;
}

/** The plan's inclusions from the room catalogue (e.g. "Breakfast", "Estate walk"). */
export function inclusionsFor(planId: string, plans: readonly Plan[]): string[] {
  return plans.find((p) => p.id === planId)?.inclusions ?? [];
}
