'use client';

import { useState } from 'react';

import { useRatesGrid, useUpdateRates, type RatePlanCode } from '@/apis/rates';
import { addDays, todayIST } from '@/shared/lib/dates';

import { PRICE_PLAN, WINDOW_DAYS } from './const';
import { nextSelection, selectedNights, weekStart } from './lib';
import type { GridSelection } from './types';

/** The fortnight on screen, the plan, and the selected nights. */
export function useRatesView() {
  const [from, setFrom] = useState(() => weekStart(todayIST()));
  const [plan, setPlan] = useState<RatePlanCode>(PRICE_PLAN);
  const [selection, setSelection] = useState<GridSelection | null>(null);
  const grid = useRatesGrid({ from, to: addDays(from, WINDOW_DAYS), plan });
  const update = useUpdateRates();

  const row = grid.data?.rows.find((r) => r.room_type_id === selection?.roomTypeId);
  const nights = selectedNights(row, selection);

  return {
    from,
    page: (weeks: number) => {
      setSelection(null);
      setFrom((f) => addDays(f, weeks * 7));
    },
    plan,
    setPlan,
    grid,
    selection,
    pick: (roomTypeId: string, date: string) => {
      update.reset();
      setSelection((s) => nextSelection(s, roomTypeId, date));
    },
    clear: () => {
      update.reset();
      setSelection(null);
    },
    row,
    nights,
    update,
  };
}
