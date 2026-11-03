/** Every page path in one place, so links and redirects can't drift apart. */
export const ROUTES = {
  home: '/',
  stays: '/#stays',
  events: '/#events',
  room: (slug: string, query?: string) => `/rooms/${encodeURIComponent(slug)}${query ? `?${query}` : ''}`,
  checkout: (holdId: string) => `/book/${encodeURIComponent(holdId)}`,
  pay: (orderId: string) => `/pay/${encodeURIComponent(orderId)}`,
  booking: (ref: string) => `/booking/${encodeURIComponent(ref)}`,
  myBookings: '/my-bookings',
  enquire: '/enquire',
  privacy: '/privacy',
  staff: {
    login: '/staff/login',
    inbox: '/staff/inbox',
    enquiry: (ref: string) => `/staff/inbox?open=${encodeURIComponent(ref)}`,
    rates: '/staff/rates',
    bookings: '/staff/bookings',
    outbox: '/staff/outbox',
    dashboard: '/staff/dashboard',
  },
} as const;
