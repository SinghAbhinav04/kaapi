import { sessionStore } from '@/shared/lib/storage';

import { STORAGE_KEY } from './const';
import type { HoldTicket } from './types';

export const holdTickets = {
  save: (ticket: HoldTicket) => sessionStore.set(STORAGE_KEY.hold(ticket.holdId), ticket),
  load: (holdId: string) => sessionStore.get<HoldTicket>(STORAGE_KEY.hold(holdId)),
  remove: (holdId: string) => sessionStore.remove(STORAGE_KEY.hold(holdId)),
};

/** After booking, the same token lets this tab read its booking (confirmation, payment). */
export const bookingTokens = {
  save: (ref: string, token: string) => sessionStore.set(STORAGE_KEY.booking(ref), token),
  load: (ref: string) => sessionStore.get<string>(STORAGE_KEY.booking(ref)) ?? undefined,
};
