'use client';

import { useRouter } from 'next/navigation';
import { useState } from 'react';

import { useCreateHold } from '@/apis/booking';
import { useAvailability, useCalendar, type Room } from '@/apis/catalog';
import { holdTickets } from '@/features/booking-flow';
import { toAvailabilityParams, type StaySearch } from '@/features/stay-search';
import { ROUTES } from '@/shared/const';
import { addMonths, firstOfMonth, type IsoDate } from '@/shared/lib/dates';

import { CALENDAR_PLAN, DEFAULT_PLAN_CODE } from './const';

/** One month of nightly prices; paging keeps the previous month on screen until the next arrives. */
export function useCalendarMonth(slug: string, startDate: IsoDate) {
  const [month, setMonth] = useState(() => firstOfMonth(startDate));
  const query = useCalendar(slug, { from: month, to: addMonths(month, 1), plan: CALENDAR_PLAN });
  return { month, setMonth, query };
}

/**
 * Everything the Reserve button needs: prices for the searched stay (every plan, with the
 * promo applied), the chosen plan, and creating the hold that freezes the price.
 */
export function useRoomBooking(room: Room, search: StaySearch) {
  const router = useRouter();
  const [promo, setPromo] = useState<string | undefined>();
  const [planCode, setPlanCode] = useState(DEFAULT_PLAN_CODE);
  const availability = useAvailability(toAvailabilityParams(search, promo));
  const option = availability.data?.options.find((o) => o.slug === room.slug);
  const plan = option?.plans.find((p) => p.code === planCode) ?? option?.plans[0];
  const createHold = useCreateHold();

  const reserve = () => {
    if (!option || !plan || option.status !== 'AVAILABLE') return;
    createHold.mutate(
      {
        room_type_id: room.id,
        rate_plan_id: plan.plan_id,
        check_in: search.checkIn,
        check_out: search.checkOut,
        rooms: search.rooms,
        adults: search.adults,
        children: search.children,
        promo_code: option.promo_error ? undefined : promo,
      },
      {
        onSuccess: ({ hold }) => {
          if (!hold.token) return;
          holdTickets.save({
            holdId: hold.id,
            token: hold.token,
            roomSlug: room.slug,
            roomName: room.name,
            planName: plan.name,
            refundable: plan.refundable,
            adults: search.adults,
            children: search.children,
            hold,
            photo: room.photos[0],
          });
          router.push(ROUTES.checkout(hold.id));
        },
      },
    );
  };

  return {
    availability,
    option,
    plan,
    planCode: plan?.code ?? planCode,
    setPlanCode,
    promo,
    setPromo,
    reserve,
    reserving: createHold.isPending || createHold.isSuccess,
    reserveError: createHold.error,
    resetReserve: createHold.reset,
  };
}
