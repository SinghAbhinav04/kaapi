'use client';

import { useRouter } from 'next/navigation';
import { useState } from 'react';

import { useCreateBooking } from '@/apis/booking';
import { STORAGE_KEY, bookingTokens, holdTickets, type HoldTicket } from '@/features/booking-flow';
import { ROUTES } from '@/shared/const';
import { useSessionValue } from '@/shared/hooks';

import { EMPTY_FORM, toBookingRequest, validateGuest } from './lib';
import type { FormErrors, GuestForm } from './types';

/** This tab's hold: undefined while reading storage, null if this tab didn't make it. */
export function useHoldTicket(holdId: string) {
  return useSessionValue<HoldTicket>(STORAGE_KEY.hold(holdId));
}

/** The details form: local state, client-side checks, then the booking and on to payment. */
export function useCheckout(ticket: HoldTicket | null | undefined) {
  const router = useRouter();
  const [form, setForm] = useState<GuestForm>(EMPTY_FORM);
  const [clientErrors, setClientErrors] = useState<FormErrors>({});
  const createBooking = useCreateBooking();

  const update = <K extends keyof GuestForm>(key: K, value: GuestForm[K]) => setForm((f) => ({ ...f, [key]: value }));

  const submit = () => {
    if (!ticket) return;
    const errors = validateGuest(form);
    setClientErrors(errors);
    if (Object.keys(errors).length > 0) return;
    createBooking.mutate(
      { body: toBookingRequest(ticket.holdId, form), token: ticket.token },
      {
        onSuccess: ({ booking }) => {
          bookingTokens.save(booking.ref, ticket.token);
          holdTickets.remove(ticket.holdId);
          if (booking.payment) router.push(ROUTES.pay(booking.payment.order_id));
          else router.push(ROUTES.booking(booking.ref));
        },
      },
    );
  };

  // Server-side field errors (same keys) override, so the message is next to the right field.
  const errors: FormErrors = { ...clientErrors, ...(createBooking.error?.fieldErrors ?? {}) };
  const formError = createBooking.error && Object.keys(createBooking.error.fieldErrors).length === 0 ? createBooking.error : null;

  return { form, update, submit, errors, formError, submitting: createBooking.isPending || createBooking.isSuccess };
}
